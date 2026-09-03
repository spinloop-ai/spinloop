## Why

A `kind: remote` fleet node only works once its AWS environment has been
deployed — but that deployment happens entirely outside `fleet.yaml`, one
environment at a time, via `spinloop remote deploy <spinloop-file>` run by
hand for each. Standing up a multi-node remote fleet today means deploying
each environment separately and then, separately again, listing them in
`fleet.yaml`. There is no single command that reads a fleet file and brings
its remote nodes into existence.

## What Changes

- Add an optional `file:` field to `kind: remote` fleet-file node entries,
  naming the Spinloop file that node deploys from (resolved relative to the
  fleet file, the way other Spinloop-relative paths already resolve). It is
  optional because a node's `name` already doubles as a lookup key, resolved
  in order when `file` is absent:
  1. the node's own `name` resolved through the existing `spinloop alias`
     registry, exactly as a bare argument to `spinloop remote deploy <name>`
     already resolves today;
  2. a subdirectory named after the node, beside the fleet file (e.g.
     `dev-1/Spinloop` beside a `fleet.yaml` naming node `dev-1`) — the same
     "a name is also a directory to look in" convention a bare `spinloop
     apply <dir>` already follows for a local Spinloop.

  A node registered with `spinloop alias add <same-name> <path>`, or simply
  laid out as `<node-name>/Spinloop` beside the fleet file, therefore needs
  no `file` field at all. `kind: daemon` nodes declare no `file` field and
  are never targeted by `fleet deploy` — not because a daemon node has no
  notion of a Spinloop file telling it what to serve (it does: routing
  already wakes an idle daemon node with a `DeployConfig` derived from
  whatever Spinloop the launch names, via `Config.Wake`/`StartWith`), but
  because that is a per-launch, dynamic push with nothing stored against the
  node, whereas `fleet deploy` persistently creates the environment a
  `kind: remote` node addresses — a step a daemon node's machine, already
  provisioned by the operator, has no equivalent of.
- Add `spinloop fleet deploy [node...]`: deploys the AWS environment for each
  named `kind: remote` node (or every `kind: remote` node in the file when
  none are named), reusing the same derivation, consent, and registration
  behavior as `spinloop remote deploy` — one node's deploy config comes from
  its own `file` field, or failing that its name resolved as an alias, or
  failing that a `<node-name>/Spinloop` beside the fleet file.
- Node deploys run independently and concurrently; one node's failure or a
  registered/live guard on it is reported against that node and does not
  stop the others.
- `--dry-run` and `--overwrite` carry the same meaning as on
  `spinloop remote deploy`, applied per node.
- `kind: daemon` nodes named on the command line, or present when no nodes
  are named, are skipped with an explanation — `fleet deploy` provisions
  cloud environments; a daemon node's machine is the operator's own.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-config`: `kind: remote` node entries gain an optional `file` path
  field naming the Spinloop file that node deploys from, falling back to
  resolving the node's own name as a registered alias, then to a
  `<node-name>/Spinloop` subdirectory beside the fleet file, when absent.
- `fleet-client`: add the `spinloop fleet deploy` command — its node
  selection, per-node deploy behavior, concurrency, and reporting.

## Impact

- `internal/fleet/config.go`: `NodeConfig` gains a `File` field
  (`yaml:"file"`), resolved relative to the fleet file's directory when set.
- `cmd/spinloop/fleet.go`: new `fleetDeployCmd`, reusing `readSpinloop`'s
  alias-then-path resolution, `deployConfigFor`, `applySpinloopEnv`, and the
  registration/consent logic factored out of `runRemoteDeploy` in
  `cmd/spinloop/remote.go`.
- `docs/commands/fleet.md` and `docs/commands/remote.md`: document the new
  field and command, and cross-reference the now-two ways to deploy a remote
  environment.
- `examples/fleet-remote/`: extend to show a deployable node.
