## Why

`spinloop remote stop` with no flag stops something. Which instance depends on
what happens to be registered as `default` — a name the user may never have
chosen, left over from before they started naming environments. The same is
true of `start`, `pause`, `restart` and `keep`: five commands that change the
state of a cloud instance while naming no target.

The danger is already on record. `remote deploy` requires `--env`, and the
change that made it so explains why: *"Creating an environment binds a name to
a machine; a silent default would hide that binding and risk clobbering the
`default` environment"*, rejecting an optional flag **as a footgun**. That
reasoning applies to stopping an instance as much as to deploying one; it was
simply applied to one command.

Two further things fall out of the same root. The default environment resolves
differently from every other name — from the registry, *or* from a superseded
`~/.config/spinloop/remote.json`, with nothing saying which answered — so code
that resolves an environment by name has to special-case one name. And the
documented environment-variable workflow, which lets the remote commands run
with no `remote.json` at all, reaches the control plane with no environment
identifier: `Config.Environment` is set only by reading a file, and the
identifier is what tells the shared Lambdas which instance to act on.

One change fixes all three, because they are the same thing: a target nobody
named.

## What Changes

- **BREAKING** Every `remote` subcommand requires `--env <name>`. There is no
  implicit target: a command that names no environment fails saying so and
  listing the registered environments, rather than acting on one the user did
  not choose. `deploy` already required it; the rest now match.
- **BREAKING** The `default` environment loses its special status. The name
  stays valid — an environment may still be called `default` — but it is
  resolved like any other name and is never assumed.
- **BREAKING** `~/.config/spinloop/remote.json` is no longer read, and its
  reader goes with it. It was the second path only the default environment
  consulted, and the reason resolving a name had a special case.
- The environment-variable workflow keeps working and starts carrying an
  identifier: where `--env <name>` names an environment with no registered
  file, a complete set of `SPINLOOP_REMOTE_*` overrides SHALL configure it,
  and **the name given is the environment identifier** sent with each control
  call. Today that path sends none.
- `remote.LoadDefault`, `remote.LoadConfig` and `remote.ConfigPath` are
  removed. Nothing resolves an environment except by name.

Fleet nodes are unaffected: a `kind: remote` node has always named its
environment by node name, which is why the fleet commands never had this
problem.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `remote-endpoint`: a `remote` subcommand names its environment explicitly;
  there is no fallback when `--env` is absent, and a command without it fails
  naming the registered environments.
- `remote-environments`: every environment resolves the same way, by name,
  from the registry; a named environment with no file may be configured by the
  `SPINLOOP_REMOTE_*` overrides, with its name as the identifier.
- `config-location`: the legacy `remote.json` is no longer one of the files
  spinloop owns under its config directory.

## Impact

- `internal/remote`: `LoadDefault`, `LoadConfig` and `ConfigPath` deleted;
  `LoadConfigFile` gains the name so it can supply the identifier when the
  file is absent.
- `cmd/spinloop/remote.go`: `resolveRemoteConfig` loses its fallback branch and
  requires the flag; every subcommand's `--env` is marked required.
- `internal/fleet`: nothing to change on main — the special case for the
  default environment exists only on the unmerged top-level-verbs branch, and
  is deleted there rather than carried.
- `docs/commands/remote.md`, `docs/env-vars.md`: the default environment stops
  being described as a fallback, and the environment-variable workflow gains
  the `--env` it always needed.
- No change to the registry layout, the control plane, or the fleet file.
