# Design: install-outfit-at-boot

## Context

See proposal.md for motivation. The constraints that shape the approach:

- The shared bake preamble (`remote/lib/image-stack.ts`) currently downloads a
  pinned outfit release, checksum-verifies it against the release's own
  `checksums.txt`, and installs it to `/usr/local/bin/outfit`; the version is
  the `OutfitVersion` Image Builder parameter, defaulting to the latest git
  release tag resolved at synth time (`latestReleaseVersion` in
  `remote/lib/config.ts`), with a synth-time guard that refuses an empty
  version (CI passes a `0.0.0-ci` placeholder for its smoke synth).
- The instance boot is one user-data script (`buildInferenceUserData` in
  `remote/lambda/start/index.ts`), shared by both runners: GPU log, swap,
  CloudWatch agent config, S3 weights sync, API-key fetch, then the runner's
  unit tail (`daemonBoot` in `remote/lambda/runners/daemon-boot.ts`) writes the
  daemon's deploy config and writes, reloads and `enable --now`s
  `outfit-daemon.service`, whose `ExecStart` is `/usr/local/bin/outfit daemon`.
  User data runs once, on the instance's first boot; a re-wake (stop/start)
  does not re-run it.
- The environment's deploy config (`remote/lambda/shared/deploy-config.ts`) is
  the per-environment "what this environment serves" document, stored in SSM,
  written by the deploy Lambda, read by the start Lambda at every wake, and
  passed into `buildInferenceUserData`. It is deliberately a statement of
  intent, and an optional field there is the established pattern (`parallel`).
- The runtime VPC has no NAT; instances get public IPs, so they have internet
  egress at boot (the S3 sync and Secrets Manager fetch already rely on it).
- Release binaries are stamped by GoReleaser with the tag minus its `v`
  (`main.version=1.26.1` for tag `v1.26.1`), and the daemon already reports
  that string in `/v1/status` (the `remote-version-reporting` capability), so
  "is this instance on the release I expect?" is answerable after the change.
- Image Builder recipes are immutable per name+version; the bake bumps
  `RUNNER_VERSION` when a component's data changes.

## Goals / Non-Goals

**Goals:**

- An outfit-only release reaches environments without an AMI bake: the AMI is
  outfit-agnostic, and a fresh boot installs the binary.
- The boot install is idempotent, checksum-verified, and atomic, and it runs
  before the daemon's unit is written and enabled.
- An environment may pin an exact outfit release via `outfit remote deploy`;
  the default is `latest`, resolved at boot.
- Rollout is safe in both directions: the new boot works on today's
  outfit-baked AMIs, and today's boot works on the new AMIs' predecessors.

**Non-Goals:**

- No `VERSION`/outfit keyword in the Outfit grammar — the pin is deployment
  state, and the Outfile stays portable (no harness, no machine-local state in
  it).
- No fleet-side pinning: `deployConfigForNode` is unchanged — fleet nodes are
  operator-owned machines whose operator installs the daemon.
- No in-session upgrade: a running or re-waking instance keeps the binary it
  booted with; a pin takes effect on the next fresh launch.
- No self-update path in the daemon itself.
- No change to the seed instance, which runs the seeder, not the daemon.

## Decisions

### D1: The pin lives in the environment's deploy config

An optional `outfitVersion` field is added to the deploy-config contract
(`DeployConfig` in `lambda/shared/deploy-config.ts`, with `outfitVersion`
normalised to `"latest"` when absent) and to the Go client's
`remote.DeployConfig` (`OutfitVersion string` with `omitempty`, so an
unpinned deploy sends a byte-identical payload to today).

- Why here: it is already the per-environment document the start Lambda reads
  at every wake and feeds to `buildInferenceUserData`, it is what `outfit
  remote deploy` writes, and it is what the deploy plan prints. Storing the
  pin there gives "a pin change takes effect on the next fresh launch" for
  free, with no new state source to keep in step.
- Alternatives considered: a separate SSM parameter or instance tag per
  environment (a second state source the deploy would have to write and the
  start Lambda reconcile — rejected); resolving `latest` to a concrete version
  at deploy time and storing that (rejected — "latest" would silently become
  "latest at deploy time", contradicting the stated default, and the deploy
  payload is deliberately a statement of intent).

### D2: `latest` is resolved at boot, in the user-data script

The boot script resolves the version, not the deploy pipeline:

```sh
OUTFIT_VERSION='<pin, or empty for latest>'
if [ -z "$OUTFIT_VERSION" ]; then
  OUTFIT_VERSION="$(curl -fsSL https://api.github.com/repos/lucinate-ai/outfit/releases/latest \
    | sed -n 's/.*"tag_name": *"\(v\{0,1\}\)\([^"]*\)".*/\2/p' | head -n1)"
fi
test -n "$OUTFIT_VERSION" || { echo "outfit version unresolved" >&2; exit 1; }
```

- A pin needs no API call — the asset URL is built directly. An unresolvable
  pin (a 404 from `curl -f`) and an unresolvable `latest` (an empty or
  rate-limited API reply) both abort the boot under `set -euxo pipefail`,
  which is the loud failure the spec requires: the instance comes up without a
  daemon, the start Lambda's poll times out with its existing `starting`
  report, and the boot log group carries the error.
- `latest` via the GitHub API (one small call per fresh boot) rather than the
  `releases/latest/download/` redirect, because the idempotency check (D3)
  needs the concrete version string and the redirect does not yield one
  without extra HEAD traffic and path parsing. The unauthenticated API rate
  limit (60/hour per source IP) is irrelevant at fresh-boot frequency.
- Version strings are the tag minus a leading `v`, matching what the stamped
  binary reports and the existing convention in `config.ts`/the bake: the
  parser strips a leading `v` from a pin, and the asset URL re-adds it
  (`releases/download/v$OUTFIT_VERSION/…`).

### D3: The install step is idempotent, verified, atomic, and shared

A new pure string-building function (in `lambda/start/index.ts`, exported for
tests, following the `buildInferenceUserData` pattern) renders the step, and
`buildInferenceUserData` emits it after the CloudWatch agent starts and before
the S3 weights sync — so a failure is captured in the boot log and aborts the
boot before the slow sync spends its minutes. It runs for both runners with no
per-runner change, ahead of the runner unit tail that writes and enables the
daemon's unit:

```sh
if [ -x /usr/local/bin/outfit ] && [ "$(/usr/local/bin/outfit version)" = "$OUTFIT_VERSION" ]; then
  echo "outfit $OUTFIT_VERSION already installed"
else
  mkdir -p /tmp/outfit-dl
  OUTFIT_URL="https://github.com/lucinate-ai/outfit/releases/download/v$OUTFIT_VERSION"
  curl -fsSL "$OUTFIT_URL/outfit_linux_amd64.tar.gz" -o /tmp/outfit-dl/outfit_linux_amd64.tar.gz
  curl -fsSL "$OUTFIT_URL/checksums.txt" -o /tmp/outfit-dl/checksums.txt
  (cd /tmp/outfit-dl && grep ' outfit_linux_amd64.tar.gz$' checksums.txt | sha256sum -c -)
  tar -xzf /tmp/outfit-dl/outfit_linux_amd64.tar.gz -C /tmp/outfit-dl
  install -m 0755 /tmp/outfit-dl/outfit /usr/local/bin/outfit
  /usr/local/bin/outfit version
  rm -rf /tmp/outfit-dl
fi
```

- Idempotent: the installed binary's own `outfit version` is compared with the
  resolved target; a match skips the download entirely.
- Verified: the same tarball-plus-`checksums.txt` `sha256sum -c` check the
  bake performs; a mismatch aborts before anything is installed.
- Atomic: GNU `install(1)` lands the binary via a temp-file-and-rename in the
  destination directory, so an interruption at any point leaves the previous
  state (no binary, or a previously verified one) in place — never a
  partial or unverified binary.
- The pin value is interpolated into generated shell, so
  `parseDeployConfig` validates its shape (a conservative character set:
  `[0-9A-Za-z.-]`, no shell metacharacters) — the same charset-guard
  rationale the companion filenames use.
- Alternative considered: a systemd oneshot unit written beside the daemon's
  (rejected — an extra unit and ordering edge for no gain; user data already
  has `set -e` fail-fast and the step must precede the daemon's unit either
  way).

### D4: CLI surface — `--outfit-version` on `outfit remote deploy` only

`outfit remote deploy` gains `--outfit-version` (long form; the pin is an
occasional operator choice, not a value worth a short flag). The value is
normalised in the CLI — a leading `v` stripped, and `latest` (or empty)
treated as "no pin", so the payload omits the field — and the plan output
prints the result as an `outfit:` line beside the runner and model:
`1.26.1` when pinned, `latest` otherwise, in both a real deploy and
`--dry-run`.

- Why deploy-only: the pin is a property of the deployment (like the runner
  and model), written once to the environment's deploy config. `remote
  start`/`restart` do not rewrite that config, so they take no flag; a pin
  change is a redeploy, which is what "takes effect on the next fresh launch"
  means.
- Alternatives considered: an Outfit keyword (rejected — the Outfile is
  portable and shared, the pin is per-deployment); a flag on `start` (rejected
  — it would either silently rewrite the stored config or be per-start state
  the boot cannot see).

### D5: The bake change is a pure deletion, behind a version bump

- `image-stack.ts`: delete the outfit block from `commonPreamble()`, the
  `OutfitVersion` parameter from both component docs and `runnerBuilds`, the
  `outfitParam`, and the synth-time empty-version guard. Bump `RUNNER_VERSION`
  for both runners (the component data changes; Image Builder recipes are
  immutable per version, and the bump is what forces a fresh AMI).
- `config.ts`: delete `outfitVersion` from `LlmConfig`, `DEFAULTS`, and
  `loadConfig`, and `latestReleaseVersion()` with it (its only consumer was
  the guard above).
- `.github/workflows/remote.yml`: delete the `-c outfitVersion=0.0.0-ci`
  smoke-synth placeholder and its now-stale comment.
- The `outfit-nudge` script, logrotate config, and CloudWatch agent install
  stay baked — they are instance plumbing, not the outfit release.

## Risks / Trade-offs

- [GitHub outage or API rate-limit at boot breaks a fresh launch] → The
  failure is loud and bounded (boot aborts, start Lambda reports `starting`
  past its deadline, the curl error is in the boot log group), and the
  operator re-runs `outfit remote start` once GitHub is reachable. This
  replaces a harder failure mode (a bake pipeline failure blocking every
  release) with a rarer, self-evident one.
- [Rollout skew: new boot on old AMI, or old boot on new AMI] → Both are safe
  in the forward direction: the new install step atomically overwrites the
  baked binary (a verified, idempotent replace), and an old boot never sees a
  new AMI before the control plane that builds the new user data is live. The
  hazardous combination (old user data on an outfit-less AMI) only arises on
  rollback — see Migration Plan.
- [`latest` moves under an unpinned environment] → That is the feature; the
  operator can pin for reproducibility, and the existing `remote
  status`/`metrics` version reporting shows what any instance actually runs.
- [A pin typo is not caught until the next fresh launch] → The deploy plan
  (and `--dry-run`) prints the normalised pin at deploy time, so a typo is
  visible before anything is sent; a genuinely non-existent version then fails
  the boot loudly rather than installing a substitute.
- [The deploy config is hand-editable SSM state] → A malformed
  `outfitVersion` in a hand-written config fails `parseDeployConfig` at the
  next wake with the parser's clear error, not at boot — the same fail-early
  behaviour the contract gives every other field.

## Migration Plan

1. Ship the control-plane change (Lambda code: deploy-config contract, user
   data with the install step) via `outfit remote bootstrap`. Current
   outfit-baked AMIs keep working: fresh launches run the new user data, which
   installs the current release atomically over the baked binary.
2. Verify at least one fresh launch per runner (status shows a version, the
   engine serves, and a pinned deploy installs exactly the pin).
3. Bump `RUNNER_VERSION`, `cdk deploy` the image stack, and `pnpm bake` both
   runners' outfit-less AMIs. The start Lambda picks the newest tagged AMI, so
   fresh launches from then on depend on the boot install.
4. Rollback, if needed: redeploy the previous control plane and re-bake from
   the previous (still-present, immutable) recipe versions, since the old user
   data expects a baked binary. The previous recipes are not deleted by the
   stack update.
