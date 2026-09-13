## Context

See proposal.md for the motivation. The implementation-relevant current state:

- `Selection.Remote` (internal/spinloop) carries the `REMOTE` value; `Parse`
  enforces it once; `Format` emits it. (The old `FLEET` instruction it was
  exclusive with is gone from the grammar on the new base.)
- Four distinct code paths read it: `applySelection`/`removeSelection`
  (provider keying + base URL, cmd/spinloop/main.go), `fetchRemoteEnv` (harness
  key fetch), `resolveRemoteConfig` (the `remote` subcommands' config
  selection), and `deriveDeployTarget` (deploy's environment name, shared by
  `remote deploy` and `fleet deploy`).
- The value is today polymorphic: a bare name resolves against the per-user
  registry (`remote.IsEnvName` + `EnvConfigPath`), a path/URL resolves against
  the Spinloop's own source via `internal/spinloopsrc`.
- `remote start` already has `-e`/`--env` as a bool flag (print export lines);
  `apply`, `unapply`, `add`, `remove`, and `harness` have no `--env` flag;
  `harness` has `-f`/`--fleet`.
- The `default` environment is already the per-user fallback (`LoadDefault`:
  `remotes/default/remote.json`, then the legacy single file, then env vars
  alone).

## Goals / Non-Goals

**Goals:**

- A Spinloop names nothing about where or under what name its model is served;
  one file deploys to many environments.
- The environment name becomes a deployment-time fact: `--env` flag for
  standalone commands, the node name for `fleet deploy`.
- The existing per-user registry and `SPINLOOP_REMOTE_*` overrides remain the
  only homes of a deployment's control config.

**Non-Goals:**

- No deprecation window for `REMOTE` (user decision: hard break).
- No change to the fleet file format, the registry layout, or the control-plane
  API; the control plane already requires an environment identifier on every
  call.
- No change to `export`/`show`'s handling of an environment-keyed provider block
  (already emits a non-re-applicable `PROVIDER <env>` today; neither improved
  nor worsened here).
- No new way to share a deployment's `remote.json` over a URL (the path/URL
  form dies; registry + env-var overrides remain).

## Decisions

**D1: The `--env` flag is the single selector, with the `default` environment
as fallback.** Every `remote` subcommand, `apply`, `unapply`, and `harness`
takes `--env <name>`; it is a registry name, never a path. Without it, the
`remote` subcommands fall back to `default` exactly as `LoadDefault` already
does. Alternatives: a `--config <path|url>` escape hatch for the path/URL form
(rejected — the user decided the form dies, and env-var overrides already cover
the manual case); deriving the name from the Spinloop path or model (rejected —
unstable and opaque, and it would reintroduce a file-derived name).

**D2: `remote deploy` requires `--env`.** Creating an environment binds a name
to a machine; a silent default would hide that binding and risk clobbering the
`default` environment. `fleet deploy` uses the node name without an override:
the node name is the registered environment's key, and the other fleet commands
look the registry up by node name, so a different name would leave the node
undriveable. Alternatives: optional `--env` defaulting to `default` (rejected
as a footgun); allowing `fleet deploy --env` to override the node name
(rejected — breaks the fleet invariant).

**D3: `start`'s export-printing flag moves to `--print-env`, no short form.**
`-e`/`--env` on `start` already exists as a bool, so `--env <name>` cannot take
it there without a rename. The `remote env` subcommand already does the same
job, so the renamed flag is a convenience, not a load-bearing surface; dropping
the short form is acceptable (it is only reachable on one subcommand).

**D4: The parser rejects `REMOTE` with a dedicated migration error.** The
generic unknown-keyword error would not name the fix, which the CLI's error
convention requires. `Parse` special-cases the one keyword anyone in the wild
will actually hit: the error names the line, says `REMOTE` was removed, and
points at `remote deploy --env <name>` and `--env <name>` on apply/harness.

**D5: `--env` conflicts with a fleet; a pinned `BASEURL` still wins for the
address.** `--env` alongside a fleet file (a `--fleet` flag or a directory
`./fleet.yaml` in force) is an error naming both — the direct successor of the
old `REMOTE`+`FLEET` exclusivity (two answers to where the model is served
from). A `BASEURL`,
however, keeps its established role as the pinned address: it supplies the
address while `--env` still provides the environment name for provider keying
and the key — exactly today's `REMOTE`+`BASEURL` semantics, generalised.
Alternatives: `--env` winning over `BASEURL` (rejected — flags overriding a
pinned address would silently discard an explicit pin); `--env`+`BASEURL` as
an error (rejected — it would break the legitimate "my gateway sits in front
of the deployment" pairing the current spec already allows).

**D6: The Spinloop argument on `remote` subcommands survives for `ENV`
loading only.** The positional Spinloop argument and the `./Spinloop`
default-consult still happen, so the Spinloop's `ENV` instructions and adjacent
`.env` (AWS credentials, `SPINLOOP_REMOTE_*` overrides) reach the control
calls — unchanged from today — but the argument no longer selects an
environment, and a Spinloop without `REMOTE`-era expectations is no longer an
error for an explicit argument.

**D7: `apply --env` against an unregistered environment is a clear error.**
The existing message shape is reused: "environment %q is not registered: run
`spinloop remote deploy --env %q` to create it". The old leniency (a path-form
`REMOTE` pointing at a not-yet-written config applied fine, base URL left to
the catalogue) is dropped: a flag is an explicit name, so a missing registry
entry is a mistake to report, not a config to wait for.

**D8: `IsEnvName` keeps its name-validation role, loses its disambiguation
role.** With no path form left, the bare-name-vs-path test is unnecessary for
resolution; the "is this a plain identifier" check remains, applied to `--env`
values so `--env ./x.json` fails saying an environment name is a plain
identifier.

## Risks / Trade-offs

- [Shared Spinloops carrying `REMOTE` break on upgrade] → the dedicated D4
  parser error names the exact fix; the change is released as a major-version
  break and the docs/examples in this change show the new flow.
- [`unapply --env` must match the `apply --env` that wrote the block, or the
  removal finds nothing] → same mechanism as today's `REMOTE` (both paths
  resolve the provider name identically); the "Nothing to remove" message
  already names the provider it looked for.
- [Users lose the URL-shared `remote.json`] → registry entries are per-machine
  by design (they hold deployment URLs and must not be committed); the
  `SPINLOOP_REMOTE_*` overrides still carry a full manual config, and a
  deployment's own output tells the user where its registered `remote.json`
  lives.
- [Two new flag spellings to learn (`--env` on most commands, `--print-env` on
  `start`)] → `--print-env` is the only long-only form in the `remote` group,
  and it exists only because the short form was taken; the export lines remain
  available from `remote env`, whose stdout is eval-safe.

## Migration Plan

No data migration: the registry layout is unchanged, and an existing
`remotes/<name>/remote.json` keeps working for every command via `--env <name>`
or, for the one-environment setup, by renaming/moving it to `remotes/default/`
or keeping the legacy `~/.config/spinloop/remote.json` fallback. Rollback is a
binary rollback; Spinloops written for the new flow (no `REMOTE` line) also
parse under the old binary only if the old build is one that still accepts the
file — they parse, since `REMOTE` was always optional — so no file is trapped
on one side.

## Open Questions

None — the naming, flag, conflict, and migration decisions were fixed with the
user during exploration (flag + fleet deploy naming; `--env/-e` on
apply/unapply/harness; `--print-env` rename; hard break; path/URL forms die).
