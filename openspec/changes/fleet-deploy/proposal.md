## Why

A `kind: remote` fleet node only works once its AWS environment has been
deployed — but that deployment happens entirely outside `fleet.yaml`, one
environment at a time, via `spinloop remote deploy <spinloop-file>` run by
hand for each. Standing up a multi-node remote fleet today means deploying
each environment separately and then, separately again, listing them in
`fleet.yaml`. There is no single command that reads a fleet file and brings
its remote nodes into existence.

## What Changes

- Add a `spinloop:` field to `kind: remote` fleet-file node entries, naming
  the Spinloop file that node deploys from (resolved relative to the fleet
  file, the way other Spinloop-relative paths already resolve). Daemon nodes
  are unaffected; the field is meaningless for `kind: daemon`.
- Add `spinloop fleet deploy [node...]`: deploys the AWS environment for each
  named `kind: remote` node (or every `kind: remote` node in the file when
  none are named), reusing the same derivation, consent, and registration
  behavior as `spinloop remote deploy` — one node's deploy config comes from
  its own `spinloop:` file exactly as a standalone `remote deploy` reads its
  Spinloop argument.
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

- `fleet-config`: `kind: remote` node entries gain an optional `spinloop:`
  path field naming the Spinloop file that node deploys from.
- `fleet-client`: add the `spinloop fleet deploy` command — its node
  selection, per-node deploy behavior, concurrency, and reporting.

## Impact

- `internal/fleet/config.go`: `NodeConfig` gains a `Spinloop` field
  (`yaml:"spinloop"`), resolved relative to the fleet file's directory.
- `cmd/spinloop/fleet.go`: new `fleetDeployCmd`, reusing `deployConfigFor`,
  `applySpinloopEnv`, and the registration/consent logic factored out of
  `runRemoteDeploy` in `cmd/spinloop/remote.go`.
- `docs/commands/fleet.md` and `docs/commands/remote.md`: document the new
  field and command, and cross-reference the now-two ways to deploy a remote
  environment.
- `examples/fleet-remote/`: extend to show a deployable node.
