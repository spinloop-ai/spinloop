## Why

A registered cloud environment and a one-node fleet are already the same
thing, reached two ways. `--env <name>` loads `remotes/<name>/remote.json`
and acts on it; a `kind: remote` fleet node named `<name>` loads the same file
through `Config.NewNode` and wraps it in a `remoteNode`. The node form gets
the fan-out, the tiled dashboard, the shared renderers and the
one-bad-node-is-a-row behaviour. The flag form gets none of that, because
nothing builds a fleet for it.

So an operator with one cloud environment and no fleet file cannot run
`spinloop fleet dashboard` against it, cannot see it beside anything else, and
must write a `fleet.yaml` naming a single node whose only content is a name
the registry already holds.

Meanwhile "which target" is asked in two places with two answers: the launch
path enforces that `--env` and `--fleet` are mutually exclusive
(`cmd/spinloop/main.go`), and the fleet commands know only about `--fleet`.
One rule stated once is the point of this change; making a registered
environment addressable as a fleet of one is what makes that rule
expressible.

## What Changes

- Every `spinloop fleet` command that resolves a fleet accepts `--env <name>`
  as an alternative to `--fleet <path>`: it acts on a fleet of one node, of
  kind remote, named by the flag. `fleet status`, `fleet metrics`, `fleet
  logs` and `fleet dashboard` therefore work against a registered environment
  with no `fleet.yaml` present.
- `fleet harness` is the one fleet command left untouched. It converges into
  `code` in a change of its own, so a flag added to it now would be surface on
  a command about to be deleted.
- One target resolver replaces the per-command `fleet.Resolve(path)` call.
  It takes what the flags say and returns the fleet to act on, so every
  command resolves its target the same way and reports the same errors.
- `--env` and `--fleet` given together SHALL fail naming both, in the wording
  the launch path already uses: each names where the model is served from, so
  state one. The same rule now holds on read commands as on a launch.
- `--env` naming an environment that is not registered SHALL fail saying so
  and how to create it — the message `remote` subcommands already give.
- The `remote` command group is untouched. `remote status --env x` and
  `fleet status --env x` both work, and neither changes what it prints. The
  overlap is deliberate and temporary: retiring the duplicate read commands
  belongs to the change that moves the read verbs to the top level.

Additive. No existing command line changes meaning, and no output changes.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `fleet-config`: the "Fleet file resolution" requirement becomes target
  resolution — a fleet command's target is `--env <name>`, `--fleet <path>`,
  or the working directory's `fleet.yaml` — plus a new requirement for what a
  fleet of one built from a registered environment is, and what it does not
  have (no file on disk, no adjacent `.env`, no fleet-wide settings).

## Impact

- `internal/fleet`: a constructor for a one-node config naming a registered
  environment. No change to `Node`, the fan-out, or the renderers — the whole
  point is that they already handle this node kind.
- `cmd/spinloop/fleet.go`, `fleet_logs.go`, `fleet_dashboard.go`: an `--env`
  flag per command, and the resolver in place of `fleet.Resolve`.
- `cmd/spinloop/main.go`: the existing `--env`/fleet exclusivity check moves
  into the shared resolver so the launch path and the fleet commands enforce
  one rule from one place.
- No change to the fleet file format, the environments registry, the control
  plane, or the `remote` command group.
- `docs/commands/fleet.md`: the new flag and what it targets.
