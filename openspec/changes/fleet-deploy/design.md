## Context

`spinloop remote deploy <spinloop-file>` already does everything a node deploy
needs: it derives a `remote.DeployConfig` from a Spinloop file
(`deployConfigFor`), resolves the target environment name from the Spinloop's
`REMOTE` instruction, guards against clobbering a registered-or-live
environment unless `--overwrite`, prints a plan (or stops there on
`--dry-run`), calls the control plane, and registers the result under
`~/.config/spinloop/remotes/<env>/remote.json`. That whole body lives in one
function, `runRemoteDeploy` in `cmd/spinloop/remote.go`, driven by a single
Spinloop path.

`spinloop fleet` (`internal/fleet`) already reads `fleet.yaml`, and a `kind:
remote` node already resolves to a registered environment by name
(`Config.NewNode`). What is missing is the link from a fleet-file node to the
Spinloop file that produces its `remote.DeployConfig`, and a command that
walks the file's remote nodes and runs a deploy for each. See `proposal.md`
for why this gap matters and `specs/fleet-config` and `specs/fleet-client` for
the resulting behavior.

## Goals / Non-Goals

**Goals:**
- One command deploys every remote node a fleet file names, without giving up
  any behavior a standalone `remote deploy` already provides for one.
- A node's deploy and a standalone `remote deploy <same-file>` can never
  disagree, because they run the same code with the same inputs.
- One node's failure or guard does not stop the rest.

**Non-Goals:**
- Provisioning `kind: daemon` nodes (installing the daemon on a bare
  machine). Out of scope per the proposal.
- Changing anything about how an already-deployed remote node is driven
  (`start`/`stop`/`status`/routing) — this only adds a path to bring the
  environment into existence.
- A new deploy Lambda contract or control-plane change. `fleet deploy` is a
  client-side batching of the same calls `remote deploy` already makes.

## Decisions

### Extract `runRemoteDeploy`'s body into a reusable function

`runRemoteDeploy(args, dryRun, overwrite, reseed, allowedCidr, region,
spinloopVersion)` currently reads its Spinloop path from `args` via
`readSpinloop` at the top and prints/returns directly. Split it into:

- `resolveDeployTarget(spinloopPath string) (sel spinloop.Selection, dc
  remote.DeployConfig, env string, err error)` — the existing
  `applySpinloopEnv` + `deployConfigFor` + `REMOTE`-name resolution, unchanged
  in behavior.
- `runDeploy(env string, dc remote.DeployConfig, opts deployOpts) deployOutcome`
  — everything from the plan print onward (the existing body from `fmt.Printf("Deploying from ...")`
  through registration), taking the already-derived `dc` and `env` rather than
  re-deriving them, and returning a value instead of writing straight to
  stdout/returning an error, so a fleet-wide caller can label each node's
  outcome instead of interleaving raw prints.

  `spinloop remote deploy` becomes a thin wrapper: derive, then call
  `runDeploy` once and print its outcome exactly as today (`deployOutcome`
  carries the same lines `runRemoteDeploy` prints now).

This is the same shape the codebase already uses for `deployConfigFor` /
`deployConfigForNode` sharing one `deployConfig` body — a derivation function
plus a target-specific wrapper — so it is consistent with the existing
pattern rather than a new one.

**Alternative considered**: have `fleet deploy` shell out to `spinloop remote
deploy` as a subprocess per node. Rejected — it would need to reconstruct
flags as argv, lose typed error handling (the per-node "guard vs. failure"
distinction the spec requires), and complicate testing (the existing tests
drive `runRemoteDeploy` through seams like `deployDiscoverFn`; a subprocess
boundary would hide those from `fleet deploy`'s tests).

### `NodeConfig.Spinloop` resolves like other Spinloop-relative paths

Add `Spinloop string \`yaml:"spinloop"\`` to `NodeConfig`. Resolved relative
to `Config.Dir` (the fleet file's own directory), the same base every other
fleet-file-relative value uses (`.env` lookup already does this). No new
resolution rule to document.

### Node selection and concurrency in `fleetDeployCmd`

```
spinloop fleet deploy [node...]
```

- No args: every `kind: remote` node in file order.
- Named args: exactly those names, in the order given; unknown name fails
  before anything is deployed (same "fail before touching the fleet" pattern
  `driveOneNode` already uses for `start`/`stop`).
- A named `kind: daemon` node fails the command outright (an explicit mistake
  worth stopping for); a `kind: daemon` node swept in only because no names
  were given is reported as skipped and otherwise ignored.

Deploys run concurrently via `errgroup`-style fan-out, mirroring
`Config.FanOut`'s shape (`internal/fleet/fanout.go`) but calling `runDeploy`
per node instead of a daemon HTTP call. Reusing `FanOut` itself is not a fit:
it is built around `Node`/`Call` (a live daemon or remote-node handle and a
read/write against it), while a deploy has no `Node` yet — deploying *creates*
what a `Node` would later address. `fleetDeployCmd` therefore builds its own
small concurrent loop, keyed by node name, collecting one outcome per node the
same shape `NodeResult` already gives fan-out callers (ok / guarded / failed),
rendered as one line per node plus a final non-zero exit when any node
failed.

### Command placement

`fleetDeployCmd` lives in `cmd/spinloop/fleet.go` beside the other fleet
subcommands, calling into `cmd/spinloop/remote.go`'s new `resolveDeployTarget`
/ `runDeploy` (same package, so no export needed). No new `internal/fleet`
dependency on `internal/remote`'s deploy internals beyond what `NewNode`
already imports.

## Risks / Trade-offs

- **Concurrent AWS calls per fleet deploy** → each node deploys a distinct
  environment (distinct Lambda invocation, distinct S3/EC2 resources), so
  there is no shared mutable state to race on; this mirrors `FanOut` already
  running concurrent calls against distinct nodes.
- **Partial success is easy to misread as full success** → the command prints
  one outcome line per node (deployed / skipped / guarded / failed) and exits
  non-zero on any failure, the same "row, not a silent gap" convention
  `fleet status` and `fleet metrics` already use for unreachable nodes.
- **A node's `spinloop` file drifts from its fleet-file entry unnoticed** →
  out of scope here; `fleet deploy`'s job is to run the deploy that file
  describes, not to detect drift. `spinloop fleet route` already gives an
  operator a way to check what a node is actually serving.

## Migration Plan

Additive only: a new optional field, a new subcommand. Existing fleet files
and existing `remote deploy` behavior are unchanged. No data migration, no
flag renames, nothing to roll back beyond reverting the change.
