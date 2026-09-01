## Context

`spinloop remote bootstrap` (cmd/spinloop/remote_bootstrap.go) downloads the
version-matched `remote/` CDK sources into a ref-keyed cache under the user
config dir (`internal/remote/source.go`), picks a package manager, prints a
consent plan, and runs: install → `cdk bootstrap` → `deploy:image` →
`bake <runner>` (per `--runners`, default both) → `deploy`, optionally
blocking on the bake with `--wait`. The bake is the slow, asynchronous,
re-runnable part (a 20–40 min Image Builder build), and it is orthogonal to
the control plane it currently rides on. `pnpm bake <runner>`
(remote/scripts/bake) already exists in the CDK project and only needs the
pipeline `deploy:image` created — no deploy of its own.

## Goals / Non-Goals

**Goals:**

- Bootstrap becomes a clean, fast, consent-gated control-plane step with no
  bake on its success path, and its success output signposts the bake.
- The bake is a first-class `spinloop remote bake` command that reuses
  bootstrap's existing source/package-manager machinery rather than
  reimplementing it.
- No change to the TypeScript CDK project.

**Non-Goals:**

- Watching the bake from `deploy`/`start` (they keep failing today when no AMI
  exists; nothing changes there).
- Any new CDK context, pipeline, or stack behaviour.
- Changing how the runtime Lambda picks up a new AMI (tags, unchanged).

## Decisions

### A new `spinloop remote bake [runner...]` command, not a printed pnpm recipe

The signpost is a command the user can run as-is, not instructions to find the
sources cache and run a package script. Bootstrap's sources land in
`<configHome>/cdk/<ref>/` — a path users do not know, which bootstrap prunes of
other refs after success, and which needs AWS credentials arranged for the
`aws` CLI anyway. A CLI command keeps the whole journey (`bootstrap` → `bake`
→ `deploy`) inside spinloop and makes the signpost one line.
Alternatives rejected: printing manual `pnpm bake` steps (burdens the user with
the cache path, the package manager, and credentials); keeping the bake on
bootstrap behind an opt-in flag (the issue asks for it not to trigger at all).

### Bake reuses bootstrap's source machinery and cache policy

Bake resolves the same version-matched ref (`remote.ResolveRef`), downloads
into the same ref-keyed default location (a present checkout is a no-op in
`DownloadRemote`), honours the same `--ref`/`--dir` and the same
`--package-manager`/`SPINLOOP_REMOTE_PACKAGE_MANAGER` resolution, and prunes
other refs after success when it used the default location — so both commands
share one cache policy instead of two that could drift. It then runs `install`
(only when `node_modules` is absent) and one `pnpm run bake <runner>` per
runner.
Alternative rejected: requiring a prior bootstrap to have left sources in
place — a newer spinloop release between the two commands would break bake,
while the download path already costs nothing when the checkout exists.

### Bake deploys nothing, and fails early when the control plane is absent

`pnpm bake` needs the Image Builder pipeline `deploy:image` created. Without a
control plane, the script's own error ("Run 'pnpm deploy:image' first") points
at a manual CDK step CLI users should not run. Bake therefore resolves AWS
credentials and checks the control-plane stack (the same
`ControlPlaneStackDeployed` call bootstrap's plan uses) before starting any
bake, and fails naming `spinloop remote bootstrap` as the first step. AWS
credentials are a hard requirement for bake in all forms: the check needs
them, and the `aws` CLI inside the bake script does. Preflight is the same
shape as bootstrap's: Node 22+, a package manager on PATH, resolvable
credentials.

### No consent gate on bake; bootstrap keeps its gate

Bootstrap's plan-and-confirm exists because it creates shared, account-level
resources with ongoing cost. Bake is the user's explicit, narrow action — they
named the runners — and touches only an Image Builder build (a builder
instance for 20–40 min). A one-line note (runners, region, expected
duration) precedes the steps; no prompt.
Alternative rejected: a confirm prompt — it would gate the very command the
issue asks to make the obvious next step.

### Runners are positional on bake, defaulting to both

`spinloop remote bake llamacpp` reads naturally in the signpost and matches
how the group's other commands take their primary noun as a positional (the
environment on `deploy`, `start`, …). Absent arguments, both runners bake —
the same default bootstrap had, so a user who wants the old end state runs one
extra command with no arguments. Validation reuses the provider→runner mapping
(`runnerFor`'s accepted set).
Alternative rejected: carrying over bootstrap's `--runners llamacpp,vllm`
flag — a flag for the command's primary input is clunkier and would leave two
spellings for the same list.

### Bootstrap loses `--runners`, `--wait`, `--force-bake`

- `--runners`: its only real effect was the bake loop. The `cdk.json`
  `context.runners` write it also performed is dead — the CDK reads no such
  key, and the image stack always creates both runners' pipelines — so the
  write goes too.
- `--wait`: nothing is left to wait for. The waiting moves to bake — and
  becomes its default behaviour, with a `--no-wait` hand-off (below);
  `waitForBake` itself is reused unchanged (polling `BakedRunners` every
  60 s under a 60-minute bound).
- `--force-bake`: with no automatic bake, "re-bake even if already
  bootstrapped" has no referent — running `bake` is already the re-bake.

The success message becomes the signpost: the account is bootstrapped; next,
`spinloop remote bake <runner>` (the bake is what an environment needs before
its first start), then `spinloop remote deploy <env>`. The plan's command list
and resource bullet drop the bakes ("Image Builder pipelines" — no "and baked
AMIs").

### Bake blocks until the AMI is available, with a `--no-wait` hand-off

The step after bake is `deploy` — and a first start needs the AMI — so the
default flow should finish at the point the user can go: bake queues the
build(s), then blocks on the same bounded `waitForBake` poll bootstrap's
`--wait` used. `--no-wait` returns as soon as the bakes are queued, reporting
how to check on them (the `pnpm bake` script already prints the build ARN and
the progress command), which keeps the documented parallelism of a bake and a
weight seed.
Alternative rejected: keeping bootstrap's opt-in `--wait` shape — it would
leave the common path handing off to the Image Builder console for 20–40
minutes.

### Shared helpers stay in package main, seams stay per-concern

`parseRunners`, `waitForBake`, the package-manager machinery
(`packageManager`, `selectPackageManager`, `resolvePackageManagerName`,
`checkNodeAndPackageManager`), the step runner, and the ref/dir/download/prune
sequence are shared between the two commands (the bootstrap-prefixed names lose
their prefix where they become shared). The test seams (the `*Fn`/`Step`
package variables) are shared too, so `remote_bake_test.go` can drive the bake
flow hermetically exactly the way `remote_bootstrap_test.go` drives bootstrap
today. `BakedRunners` stays in `internal/remote` (comment updated); the CDK
project is untouched.

## Risks / Trade-offs

- **Scripts calling `bootstrap --runners/--wait/--force-bake` break** →
  BREAKING by intent (issue #139). pflag rejects the removed flags with an
  unknown-flag error naming nothing; release notes and the updated docs carry
  the `bake` replacement, and bootstrap's own success output now says what to
  run next.
- **A new-account journey is one step longer** → the point of the change.
  `deploy`/`start` with no AMI present fail as they do today; the signpost
  puts `bake` in front of `deploy` so the order is discoverable.
- **Bake's stack check is one extra AWS call per run** → a single
  `DescribeStacks`, negligible against a 20–40-minute build.
- **Bake pruning other ref caches could surprise a user juggling binary
  versions** → same policy bootstrap already applies on success; an explicit
  `--dir` opts out, and a re-download of a pruned ref is cheap.
- **A failed bake only surfaces as the 60-minute wait timeout** —
  `BakedRunners` sees available AMIs, not failed builds, so a broken driver
  install waits out the bound before reporting. → The bake script's streamed
  output (build ARN, progress command) is on the terminal the whole time, so
  the operator can see the failure in parallel; failing fast on the build
  state is a follow-up, not part of this split.

## Migration Plan

No data or infrastructure migration — pure CLI surface. Rollback is reverting
the change; nothing on AWS is affected by either direction (accounts that
already bootstrapped keep their pipelines and AMIs).

## Open Questions

None — the spec-level behaviour is settled by the proposal and deltas.
