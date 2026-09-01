## Why

`spinloop remote bootstrap` currently kicks off the AMI bakes (both runners by
default) as part of the once-per-account control-plane setup. A bake is slow
(~20–40 min), asynchronous, re-runnable, and sometimes unnecessary — yet it is
wired into bootstrap's success path, its consent plan, and its flags
(`--runners`, `--wait`, `--force-bake`). The control plane should be a clean,
fast, self-contained step; the bake is a separate concern that the user should
choose to run (issue #139).

## What Changes

- New `spinloop remote bake [runner...]` command: starts an AMI bake for each
  requested runner (default: both `llamacpp` and `vllm`). It reuses bootstrap's
  CDK-source machinery — the same version-matched download, ref-keyed cache,
  package-manager selection, `--ref`/`--dir`/`--package-manager` flags — and
  runs `pnpm bake <runner>` per runner. Bake blocks until the baked AMI(s) are
  available; a `--no-wait` flag returns as soon as the bakes are queued,
  reporting how to check on them.
- **BREAKING** `spinloop remote bootstrap` no longer starts any AMI bake: the
  `--runners`, `--wait`, and `--force-bake` flags are removed, the sequence is
  just install → `cdk bootstrap` → `deploy:image` → `deploy`, and the consent
  plan no longer lists bakes. Bootstrap's success output signposts
  `spinloop remote bake` as the next step, ahead of `spinloop remote deploy`.
- Docs follow: the bootstrap section of `docs/commands/remote.md` and
  `remote/README.md` describe the split flow.

## Capabilities

### New Capabilities

(none — the bake lives under the existing provisioning capability)

### Modified Capabilities

- `endpoint-provisioning`: bootstrap stops baking AMIs (and loses
  `--runners`/`--wait`/`--force-bake`); new requirement defining
  `spinloop remote bake` as the separate, signposted bake command with its own
  `--wait` and the shared source/package-manager machinery.
- `remote-endpoint`: the remote command group's subcommand list gains `bake`.

## Impact

- `cmd/spinloop/remote_bootstrap.go` — flag and sequence removal, plan and
  success-message changes; `cmd/spinloop/remote_bake.go` (new) — the bake
  command; shared helpers (`parseRunners`, `waitForBake`, source resolution)
  move to be used by both.
- `internal/remote/bake.go` — `BakedRunners` now serves `bake --wait` instead
  of `bootstrap --wait` (comment change only).
- Tests: `cmd/spinloop/remote_bootstrap_test.go` updated, `remote_bake_test.go`
  new; coverage stays ≥ 80%.
- Docs: `docs/commands/remote.md`, `remote/README.md`, `docs/env-vars.md`.
- No change to the TypeScript CDK project in `remote/` — `pnpm bake` already
  exists and is what the new command drives.
- User journey for a new account: bootstrap → bake → deploy (previously
  bootstrap → deploy, with the bake a side effect of bootstrap). Accounts that
  already bootstrapped with an older binary have their AMIs and are unaffected.
