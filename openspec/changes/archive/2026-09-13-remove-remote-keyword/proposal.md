## Why

A Spinloop's `REMOTE` instruction embeds the name of one remote environment into a
file that is meant to be shared and reused: a Spinloop can be used by more than
one remote, so pinning it to a single environment constrains the file for nothing.
The value is also machine-local (a bare name resolves against the running
machine's per-user registry) yet sits in a file that travels by URL — the same
category of violation as the harness and the alias, which Spinloops already never
name. `fleet deploy` exposes the redundancy concretely: the fleet node is named
`qwen`, and the node's Spinloop must say `REMOTE qwen` again. We moved away from
naming shared resources in Spinloops before; this finishes the job for `REMOTE`.

## What Changes

- **BREAKING**: the `REMOTE` instruction is removed from the Spinloop grammar. A
  `REMOTE` line now fails to parse with an error that says what replaces it: name
  the environment with `remote deploy --env <name>` and pass `--env <name>` to the
  commands that act on it. There is no deprecation window.
- **BREAKING**: the path and URL forms of a remote configuration
  (`REMOTE ./remote.json`, `REMOTE https://…/remote.json`) die with the keyword.
  A remote environment's control config is reachable only through the per-user
  registry (`~/.config/spinloop/remotes/<name>/remote.json`) and the existing
  `SPINLOOP_REMOTE_*` overrides.
- `spinloop remote deploy` names the environment it creates with a required
  `--env <name>` flag. `spinloop fleet deploy` names it with the node's own name —
  the existing invariant that a `kind: remote` node's name is the registered
  environment it drives — and takes no override.
- Every `remote` subcommand selects its environment with a new `--env <name>`
  flag (registry lookup); with no flag, the `default` environment is used, as the
  per-user fallback already is. `remote start`'s existing `-e`/`--env` bool
  (print export lines) is renamed to `--print-env`, with no short form, freeing
  `--env`.
- `apply`, `unapply`, and `harness` take a new `--env/-e <name>` flag carrying
  what the `REMOTE` instruction used to: the harness provider is keyed on the
  environment name, the base URL is taken from the environment's registered
  `remote.json` when the Spinloop states no `BASEURL`, the provider is labelled
  with the environment, and `harness` fetches and injects the endpoint's key and
  base URL as it does today. `--env` alongside a fleet file (a `--fleet` flag or
  a `./fleet.yaml` in force) is an error naming both, the successor of the old
  `REMOTE`+`FLEET` exclusivity; a pinned `BASEURL` still supplies the address,
  as it does for routing.
- A Spinloop named by an argument to a `remote` subcommand no longer selects
  anything; the argument remains so the Spinloop's `ENV` instructions and
  adjacent `.env` are applied before AWS work, exactly as today.
- Documentation and examples follow: the `REMOTE` line disappears from
  `examples/fleet-remote`'s Spinloops (the duplicated names vanish), and the docs
  describe the flag-based flow.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `spinloop-files`: `REMOTE` leaves the keyword list and the "naming a remote
  endpoint" scenario; a new requirement makes a `REMOTE` line fail with a
  migration error naming the replacement.
- `remote-endpoint`: the "Remote configuration discovery" requirement now selects
  the environment by `--env` flag, falling back to the `default` environment;
  the path/URL-resolution scenarios and the "explicit Spinloop without a REMOTE
  fails" scenario are removed; the Spinloop argument is no longer a selector.
- `remote-environments`: the "Resolving a REMOTE value to an environment or a
  file" requirement is removed (registry lookup by name is what remains);
  "Environment name validity" now validates the `--env` value; "A REMOTE names
  the harness provider" and "A remote harness provider is labelled distinctly"
  now key off the `--env` flag; the registry's purpose and storage requirements
  no longer describe a name travelling in a Spinloop.
- `environment-deployment`: `remote deploy` names its environment with the
  required `--env` flag; `fleet deploy` uses the node name; the "Spinloop
  stating REMOTE prod resolves to it" scenario is rewritten around the flag.
- `provider-selection`: the base-URL-from-remote-configuration part of "Base URL
  precedence" now applies to an `apply` given `--env`; the scenarios referencing
  `REMOTE` are rewritten around the flag, keeping BASEURL-wins and
  not-yet-deployed-is-not-an-error.
- `harness-remote-env`: the launch fetches the endpoint's environment when
  `--env/-e` is given (instead of when the Spinloop contains `REMOTE`); the
  conflict with a fleet is specified; the "harness without REMOTE is
  unaffected" scenario becomes "without `--env`".
- `remote-env`: `start`'s `-e`/`--env` export-printing flag is renamed
  `--print-env` with no short form; the `env` subcommand's resolution scenarios
  follow the new flag-based selection.
- `remote-spinloop-sources`: the path-form `REMOTE` is no longer a Spinloop-family
  reference; the capability covers the Spinloop file and `PRESET`.
- `fleet-routing`: wording that references a `REMOTE` endpoint's address slot is
  updated; a launch given `--env` alongside a fleet (instruction or flag) fails
  naming both.
- `alias-registry`: the `SPINLOOP_ALIAS` scenario that selected an endpoint via a
  Spinloop's `REMOTE` is rewritten around the `ENV`-loading role the Spinloop
  keeps.
- `remote-local-environment`: the commands' Spinloop resolution no longer
  selects an environment; the requirement's framing is updated to the
  `ENV`-loading role.
- `remote-stats`: "requires the Spinloop to name a REMOTE environment" becomes
  `--env` selection with the `default` environment as fallback.
- `remote-logs`: the environment-selection rule ("explicit Spinloop's REMOTE,
  else default") becomes `--env`, else default.
- `remote-keep`: the same resolution rule is updated.

## Impact

- `internal/spinloop`: `Selection.Remote`, the `kwRemote` constant, and the
  `Format` line are removed; the parser gains a dedicated `REMOTE` rejection.
- `cmd/spinloop`: `applySelection`/`removeSelection`/`fetchRemoteEnv` take the
  environment name from a flag; `resolveRemoteConfig`, `resolveRemotePath`,
  `remoteEnvName`, `remoteBaseURL`, and `resolveRemoteConfigForSpinloop` collapse
  to a registry lookup by name; `deriveDeployTarget` stops reading the name from
  the Spinloop; `remote start`'s flag is renamed; `apply`/`unapply`/`harness`
  gain `--env/-e`; `fleet deploy` passes the node name through.
- `internal/remote`: `IsEnvName`'s bare-name-vs-path disambiguation role ends
  (its name-validation role for `--env` values remains); the URL-fetch path for
  remote configs is dropped.
- `internal/spinloopsrc` survives for `PRESET` and Spinloop-file fetches.
- Docs (`docs/spinloop-file.md`, `docs/commands/remote.md`, `docs/commands/apply.md`,
  `docs/commands/fleet.md`, `docs/README.md`, `docs/env-vars.md`) and
  `examples/fleet-remote` are updated.
- Shared Spinloops in the wild that carry a `REMOTE` line will fail to parse
  until the line is removed — the dedicated parser error names the fix.
