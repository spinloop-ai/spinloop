# Proposal

## Why

`--env` reads as "environment variables" — which on this CLI genuinely means
the `ENV` instructions, `.env` files, and `cloud env` exports — but the flag
selects something else entirely: a deployment target. Our internal domain
language for a deployment target is already **node** (`fleet.Node`, and a
registered environment is modelled as a fleet of one node), so the external
surface should say the same word. Closes #236.

## What Changes

- **BREAKING**: `--env` and its `-e` short form are removed from every command
  that took them (`apply`, `unapply`, `harness open`/`code`, `status`,
  `metrics`, `logs`, `dashboard`, the `fleet` subcommands that target one
  environment, and every `cloud` subcommand). They are replaced by
  `--node`/`-n`. There is no deprecated alias — the rename is clean.
- On the launch path (`harness open`/`code`) the renamed flag **merges with the
  existing fleet-node pin** `--node`, which keeps its name: one flag, one
  meaning — "talk to this node". With a fleet in force (a `--fleet` file or a
  discovered `./fleet.yaml`) the name is a node in that fleet, exactly as the
  pin works today; with no fleet in force the name is a registered
  environment — a fleet of one — exactly as `--env` worked today. The old
  `--env`+fleet conflict error disappears: naming both is now how you pin.
- All user-facing strings that name the flag or its target follow: help text,
  errors (registry, validation, conflict), completion (`--env` completions and
  the launch slot move to `--node`, which also offers fleet nodes when a fleet
  is in scope), and the docs site.
- Unchanged, because these are about environment variables and the name is
  correct: the Spinloop `ENV` instruction, `.env` files, `spinloop cloud env`,
  `--print-env`, `--token-env`. The internal `cloud` package keeps its
  `Environment` vocabulary for the registry; only the CLI's external surface
  speaks "node".

## Capabilities

### New Capabilities

None — this renames the surface existing capabilities already govern.

### Modified Capabilities

- `harness-remote-env`: the launch flag becomes `--node/-n`; key-injection and
  auto-configure requirements follow the rename.
- `fleet-routing`: the pin flag is the one `--node/-n`; the `--env`-vs-fleet
  conflict scenario becomes pin semantics, and a bare `--node` with no fleet
  resolves as the registered environment.
- `code-command`: the launch flag list and the `--env` scenario rename.
- `fleet-config`: the single-environment target flag becomes `--node`; conflict
  wording, discovery, and fleet-of-one scenarios rename.
- `fleet-client`: scenarios naming `status/metrics/logs/dashboard --env`
  rename.
- `remote-environments`: flag spelling, name validation and registry errors
  speak `--node`.
- `remote-endpoint`: `cloud` subcommand selection flag and its error wording
  rename.
- `remote-env`: scenarios rename; the `--print-env` rationale that cites the
  former `--env` spelling updates.
- `remote-start-probe`: the stale `if --env is set` becomes `--print-env`.
- `environment-deployment`: `deploy` names its environment with `--node`.
- `cloud-command`: scenario rename.
- `provider-selection`: apply-time `--env` requirement renames.
- `spinloop-files`: the removed-key guidance names `--node` as the replacement.
- `remote-stats`: `metrics` selection flag renames.
- `remote-logs`: `logs` selection flag renames.
- `remote-keep`: `keep` selection flag renames.
- `remote-schedule`: `schedule` selection flag renames.
- `remote-version-reporting`: `status`/`metrics` scenarios rename.
- `remote-local-environment`: the default-environment fallback wording renames.

## Impact

- `cmd/spinloop/`: every `StringVar*(…, "env", "e", …)` becomes
  `"node", "n"` (route.go's `routeOptions` merges `envName` into `node`; the
  launch branch in `code.go` re-decides environment-vs-fleet by whether a fleet
  resolves; `applyRoutedSpinloop` drops `checkOneTarget` and keys the cloud
  fetch/apply off the resolved environment only when no fleet answered);
  `target.go` (`fleetTarget.envName` → `nodeName`, message wording);
  `complete.go` (flag completion and `launchSlot`/`launchValueFlags` for
  `--node`/`-n`).
- Tests across `cmd/spinloop/` that pass `--env`/`-e` or assert their messages.
- Specs above, plus `docs/` and `examples/` prose the docs-update step covers.
- No change to config file formats, the registry layout on disk, the daemon
  API, or `remote/` (TypeScript control plane).
