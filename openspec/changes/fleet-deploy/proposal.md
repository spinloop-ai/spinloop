## Why

A fleet-file node has no declared link to the Spinloop file that says what it
runs. For a `kind: remote` node this means its AWS environment can only be
brought into existence outside `fleet.yaml` entirely, one environment at a
time, via `spinloop remote deploy <spinloop-file>` run by hand. For a `kind:
daemon` node it means `spinloop fleet start <node>` can only start whatever
the daemon already happens to be configured to run — it has no way to say
"start this node serving what this Spinloop names," the way a routed launch
already can via `Config.Wake`/`StartWith` for the Spinloop it happens to be
launching. Standing up a multi-node remote fleet today means deploying each
environment separately and then, separately again, listing them in
`fleet.yaml`; naming what a daemon node should run means editing that
node's own local configuration rather than the fleet file.

## What Changes

- Add an optional `file:` field to a fleet-file node entry (either kind),
  naming the Spinloop file that describes what the node runs. The path
  resolves relative to the fleet file, the way other Spinloop-relative paths
  already resolve. It is optional because a node's `name` already doubles as
  a lookup key, resolved in order when `file` is absent:
  1. the node's own `name` resolved through the existing `spinloop alias`
     registry, exactly as a bare argument to `spinloop remote deploy <name>`
     already resolves today;
  2. a subdirectory named after the node, beside the fleet file (e.g.
     `dev-1/Spinloop` beside a `fleet.yaml` naming node `dev-1`) — the same
     "a name is also a directory to look in" convention a bare `spinloop
     apply <dir>` already follows for a local Spinloop.

  A node registered with `spinloop alias add <same-name> <path>`, or simply
  laid out as `<node-name>/Spinloop` beside the fleet file, therefore needs
  no `file` field at all.
- Add `spinloop fleet deploy <node...>` (or `--all`): deploys the AWS
  environment for each named `kind: remote` node, or every `kind: remote`
  node with `--all`, reusing the same derivation, consent, and registration
  behavior as `spinloop remote deploy` — one node's deploy config comes from
  its resolved Spinloop source. Naming no node and passing no `--all` fails,
  listing the fleet's `kind: remote` nodes, rather than silently deploying
  the whole fleet — the same "an explicit target is required" rule
  `start`/`stop` already enforce for mutating fleet commands. Naming a
  `kind: daemon` node fails, explaining that `fleet deploy` provisions cloud
  environments and that node is not one; `--all` only ever selects `kind:
  remote` nodes, so a daemon node is never swept in by it.
- Node deploys run independently and concurrently; one node's failure or a
  registered/live guard on it is reported against that node and does not
  stop the others. `--dry-run` and `--overwrite` carry the same meaning as
  on `spinloop remote deploy`, applied per node.
- **BREAKING**: `spinloop fleet start <node>` on a `kind: daemon` node now
  requires that node's Spinloop source to resolve, the same way `fleet
  deploy` requires one for a `kind: remote` node. The client derives a
  deploy config from it (`deployConfigForNode`, the same derivation a routed
  wake already uses) and starts the node's engine with it via `StartWith`,
  exactly as a routed launch wakes a node — telling the daemon what to run
  rather than trusting it already knows. A `kind: daemon` node with no
  resolvable source fails `fleet start` for that node, naming the three ways
  one could have been given, rather than falling back to a plain,
  config-less start. Every fleet file with a `kind: daemon` node needs a
  `file` field, a matching alias, or a matching subdirectory added before
  `fleet start` works on it again. A `kind: remote` node's start is
  unaffected — what it serves is fixed at deploy time, and its `StartWith`
  already refuses a deploy config for that reason.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-config`: a fleet-file node (either kind) gains an optional `file`
  path field naming the Spinloop file that describes what it runs, falling
  back to resolving the node's own name as a registered alias, then to a
  `<node-name>/Spinloop` subdirectory beside the fleet file, when absent.
- `fleet-client`: add the `spinloop fleet deploy` command (node selection,
  per-node deploy behavior, concurrency, reporting), and modify `spinloop
  fleet start` to require and use a `kind: daemon` node's resolved Spinloop
  source (**BREAKING** for a node with none).

## Impact

- `internal/fleet/config.go`: `NodeConfig` gains a `File` field
  (`yaml:"file"`), resolved relative to the fleet file's directory when set.
- `cmd/spinloop/fleet.go`: new `fleetDeployCmd`; `fleetStartCmd`/
  `driveOneNode` require and use the resolved source for daemon nodes,
  always via `StartWith`. Both reuse `readSpinloop`'s alias-then-path
  resolution, `deployConfigFor`/`deployConfigForNode`, `applySpinloopEnv`,
  and (for deploy) the registration/consent logic factored out of
  `runRemoteDeploy` in `cmd/spinloop/remote.go`.
- `docs/commands/fleet.md` and `docs/commands/remote.md`: document the new
  field, its fallbacks, the new command, and `start`'s new requirement.
- `examples/fleet-remote/`, `examples/fleet-local/`, `examples/fleet-docker/`,
  `examples/fleet-mixed/`: every example with a `kind: daemon` node needs a
  `file` field, alias, or subdirectory added, or `fleet start` breaks for it.
