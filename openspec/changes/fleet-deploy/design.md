## Context

`spinloop remote deploy <spinloop-file>` already does everything a
`kind: remote` node's deploy needs: it derives a `remote.DeployConfig` from a
Spinloop file (`deployConfigFor`), resolves the target environment name from
the Spinloop's `REMOTE` instruction, guards against clobbering a
registered-or-live environment unless `--overwrite`, prints a plan (or stops
there on `--dry-run`), calls the control plane, and registers the result
under `~/.config/spinloop/remotes/<env>/remote.json`. That whole body lives
in one function, `runRemoteDeploy` in `cmd/spinloop/remote.go`, driven by a
single Spinloop path.

Separately, `internal/fleet/wake.go`'s `Config.Wake` already derives a
`remote.DeployConfig` too (via `deployConfigForNode`, the node-owned variant:
no forced context size, the preset's bind survives) and pushes it to a node
with `Node.StartWith` — but only when a routed launch wakes an idle node to
serve *that launch's* Spinloop. Nothing is stored against the node; the
config is recomputed fresh every time from whatever is being launched. A
`daemonNode.StartWith` forwards it to the daemon; a `remoteNode.StartWith`
always refuses — a remote environment's model is fixed for good at deploy
time, not pushed at wake time (`internal/fleet/remote_node.go:77-83`).

`fleet.yaml` (`internal/fleet/config.go`) has no field connecting a node to a
Spinloop file at all today. That is the actual gap two different fleet
commands hit:

- A `kind: remote` node's environment can only be created outside the fleet
  file, one `remote deploy <file>` at a time.
- A `kind: daemon` node's `fleet start <node>` has no way to say what that
  node should run — it can only start whatever the daemon is already
  configured with, unlike a routed wake, which always knows because it is
  driven by the Spinloop being launched, not by the node.

Both gaps are closed by the same fix: give a node a resolvable link to a
Spinloop file, then let each command use it the way it already knows how to
use a `DeployConfig` — `fleet deploy` via `deployConfigFor` (cloud-owned,
persistent), `fleet start` via `deployConfigForNode` + `StartWith` (node-owned,
one wake), exactly `Config.Wake` already does per launch. See `proposal.md`
for why this gap matters and `specs/fleet-config` and `specs/fleet-client` for
the resulting behavior.

## Goals / Non-Goals

**Goals:**
- One command deploys every named remote node a fleet file names, without
  giving up any behavior a standalone `remote deploy` already provides for
  one.
- A node's resolved Spinloop source and a standalone `remote deploy
  <same-file>`/`<same-alias>` can never disagree, because both run through
  the same resolution and the same derivation code.
- `fleet start` on a `kind: daemon` node uses the same resolved source to
  tell the daemon what to run, the same way a routed wake already does for
  the Spinloop being launched — and requires it, exactly as `fleet deploy`
  requires one for a `kind: remote` node. No unreleased tool has existing
  users to protect, so there is no fallback path to design around: one
  resolution mechanism, required everywhere it is the only source of truth.
- One node's deploy failure or guard does not stop the others.

**Non-Goals:**
- Provisioning `kind: daemon` nodes (installing the daemon on a bare
  machine). Out of scope per the proposal — this only tells an
  already-running daemon what to run.
- Changing how a routed launch (`spinloop harness`) wakes a node. That path
  keeps deriving its `DeployConfig` from the Spinloop being launched, exactly
  as today; a node's own resolved source is a separate, independent input
  used only by `fleet start` run directly.
- Changing anything about how an already-deployed remote node's environment
  itself is driven (`stop`/`status`), or how a `kind: remote` node's `start`
  behaves — it always uses a plain start, resolved source or not.
- A new deploy Lambda contract or control-plane change. `fleet deploy` is a
  client-side batching of the same calls `remote deploy` already makes.

## Decisions

### Extract `runRemoteDeploy`'s body into reusable functions

`runRemoteDeploy(args, dryRun, overwrite, reseed, allowedCidr, region,
spinloopVersion)` currently resolves its Spinloop argument via `readSpinloop`
(which itself tries `resolveAlias` before treating the argument as a literal
path or URL) and prints/returns directly. Split it into:

- `deriveDeployTarget(spinloopArg string) (sel spinloop.Selection,
  spinloopPath string, dc remote.DeployConfig, env string, err error)` — the
  existing `readSpinloop` (alias-or-path resolution) + `applySpinloopEnv` +
  `deployConfigFor` + `REMOTE`-name resolution, unchanged in behavior. Taking
  the raw, unresolved argument (rather than an already-resolved path) is what
  lets `fleet deploy` hand it a node's bare name and get the same alias
  resolution a standalone `remote deploy <name>` gets.
- `runDeploy(env string, dc remote.DeployConfig, opts deployOpts) deployOutcome`
  — everything from the plan print onward (the existing body from `fmt.Printf("Deploying from ...")`
  through registration), taking the already-derived `dc` and `env` rather than
  re-deriving them, and returning a value instead of writing straight to
  stdout/returning an error, so a fleet-wide caller can label each node's
  outcome instead of interleaving raw prints.

  `spinloop remote deploy` becomes a thin wrapper: derive, then call
  `runDeploy` once and print its outcome exactly as today.

This is the same shape the codebase already uses for `deployConfigFor` /
`deployConfigForNode` sharing one `deployConfig` body — a derivation function
plus a target-specific wrapper — so it is consistent with the existing
pattern rather than a new one.

**Alternative considered**: have `fleet deploy` shell out to `spinloop remote
deploy` as a subprocess per node. Rejected — it would need to reconstruct
flags as argv, lose typed error handling (the per-node "guard vs. failure"
distinction the spec requires), and complicate testing.

### A node's Spinloop source resolves the same way for every consumer

Add `File string \`yaml:"file"\`` to `NodeConfig`, available on either node
kind. This is deliberately *not* a new resolution mechanism: a node's `name`
is already a key `spinloop alias` can map to a Spinloop file, and
`readSpinloop` already turns a directory argument into `<dir>/Spinloop`
(`cmd/spinloop/main.go`'s `os.Stat` + `IsDir` check ahead of `os.ReadFile`,
the same join `spinloop apply <dir>` relies on today) — so a node whose name
matches an existing alias, or that simply has a same-named subdirectory
beside the fleet file, needs no `file` field at all.

A shared helper resolves one node to one argument, tried in order and
stopping at the first that resolves:

```go
func resolveNodeSpinloop(node fleet.NodeConfig, fleetDir string) (arg, source string, err error)
```

1. `file` set → resolve it relative to `fleetDir` into a path, and use that.
   A real path never matches an alias name, so `readSpinloop` (called next,
   inside `deriveDeployTarget`) treats it as the literal Spinloop file (or
   URL) to read, exactly as an explicit argument to `remote deploy <file>`
   would.
2. `file` unset → check the node's own `Name` against the alias registry
   (`config.Load().Alias(name)`, the same lookup `resolveAlias` makes) — a
   hit means `Name` becomes the argument, so `readSpinloop`'s own
   `resolveAlias` step resolves it again the same way a standalone `remote
   deploy <name>` would (printing the same "Using alias …" line), rather
   than this code pre-resolving the path itself and skipping that step.
3. No alias named after the node → check whether `<fleetDir>/<Name>` exists
   as a directory; a hit means that directory becomes the argument, and
   `readSpinloop`'s own directory join finds `<fleetDir>/<Name>/Spinloop`
   inside `deriveDeployTarget`.
4. None of the three resolve → `resolveNodeSpinloop` returns an error naming
   all three: no `file` field, no alias named `<Name>`, no `<Name>/`
   subdirectory beside the fleet file.

Trying the alias registry before the subdirectory matches the existing
precedence in `resolveAlias` itself, where a registered name is consulted
before anything is looked for on disk. Steps 2 and 4 need one read of the
alias registry to decide *whether* to try passing `Name` through — that read
is unavoidable because the caller needs to know whether to fall through to
the subdirectory check, not just call `deriveDeployTarget` once and inspect
the error: `readSpinloop`'s own literal-path fallback after a failed alias
lookup would otherwise silently resolve `Name` against the *current working
directory* rather than the fleet file's directory, the wrong base.

`resolveNodeSpinloop` is shared by both consumers, and both treat step 4 the
same way: a hard per-node failure (see the next two decisions). Neither
falls back to acting without a resolved source.

**Alternative considered**: make `file` required whenever no alias named
after the node exists, dropping the subdirectory convention. Rejected per
the request to support a fleet laid out as one subdirectory per node
(`fleet.yaml` beside `dev-1/Spinloop`, `dev-2/Spinloop`, …) with zero
per-node configuration beyond the node's own name.

### `fleet deploy` requires an explicit target

```
spinloop fleet deploy <node...>
spinloop fleet deploy --all
```

No node and no `--all` fails, listing the fleet's `kind: remote` nodes,
deploying nothing — the same rule `driveOneNode` already enforces for
`start`/`stop`: a command that creates or mutates cloud resources for
however many nodes are listed must never do so by accident because the
operator forgot an argument. `--all` and explicit node names together is
rejected as ambiguous. Named args resolve to exactly those nodes, in the
order given; an unknown name fails before anything is deployed. A named
`kind: daemon` node fails the command outright. `--all` selects every `kind:
remote` node and nothing else — a `kind: daemon` node is never a candidate
for it, so there is nothing to skip or report for that case.

For each targeted node, `resolveNodeSpinloop` runs; a node for which nothing
resolves fails for that node alone, without touching the other targeted
nodes (see fleet-client's "derives and applies each node's config"
requirement).

Deploys run concurrently, mirroring `Config.FanOut`'s shape
(`internal/fleet/fanout.go`) but calling `runDeploy` per node instead of a
daemon HTTP call. Reusing `FanOut` itself is not a fit: it is built around
`Node`/`Call` (a live daemon or remote-node handle and a read/write against
it), while a deploy has no `Node` yet — deploying *creates* what a `Node`
would later address. `fleetDeployCmd` therefore builds its own small
concurrent loop, keyed by node name, collecting one outcome per node the
same shape `NodeResult` already gives fan-out callers (ok / guarded /
failed), rendered as one line per node plus a final non-zero exit when any
node failed.

### `fleet start` requires a daemon node's resolved source

`driveOneNode`'s call closure currently only receives the live `fleet.Node`:

```go
func(ctx context.Context, n fleet.Node) fleet.NodeResult
```

`fleetStartCmd`'s closure needs the resolved `NodeConfig` and the fleet's
directory too, to resolve a source before starting. `driveOneNode` gains
those to its call signature:

```go
func(ctx context.Context, cfg *fleet.Config, entry fleet.NodeConfig, n fleet.Node) fleet.NodeResult
```

`fleetStopCmd`'s closure ignores the additions — stopping needs no config.

`fleetStartCmd`'s closure, for a `kind: daemon` entry: call
`resolveNodeSpinloop`; on failure, fail that node's start, naming the three
ways a source could have been given, exactly as `fleet deploy` fails an
unresolved `kind: remote` node. On success, `readSpinloop` +
`applySpinloopEnv` + `deployConfigForNode` (the node-owned derivation
`Config.Wake` already uses — no forced context size, the preset's bind
survives) to get a `dc`, print which source resolved and what it derived
(the same transparency `fleet deploy` gives), then `n.StartWith(ctx, &dc,
engineKey)`. A `kind: remote` entry always uses a plain `n.Start(ctx)`
regardless of whether a source resolves for it — `StartWith` refuses a
config for that kind unconditionally, so there is nothing to resolve for.

**Alternative considered**: fall back to a plain, config-less `Start` when
nothing resolves, so a fleet file with no `file`/alias/subdirectory for a
node keeps working. Rejected: that fallback exists only to protect a user
of today's `fleet start` who has not adopted this field, and there is no
such user yet — carrying the fallback would mean permanently maintaining two
start paths (config-less and config-driven) for a distinction that only
matters during a migration nobody needs to make. A `kind: daemon` node
without a resolvable source is a fleet-file omission to fix, the same as an
undeployed `kind: remote` node is.

### Command placement

`fleetDeployCmd` lives in `cmd/spinloop/fleet.go` beside the other fleet
subcommands, calling into `cmd/spinloop/remote.go`'s new
`deriveDeployTarget` / `runDeploy` and the new `resolveNodeSpinloop` helper
(same package, so no export needed); `fleetStartCmd`'s changed closure calls
the same `resolveNodeSpinloop` and `deployConfigForNode`. No new
`internal/fleet` dependency on `internal/remote`'s deploy internals beyond
what `NewNode` already imports.

## Risks / Trade-offs

- **Concurrent AWS calls per fleet deploy** → each node deploys a distinct
  environment (distinct Lambda invocation, distinct S3/EC2 resources), so
  there is no shared mutable state to race on; this mirrors `FanOut` already
  running concurrent calls against distinct nodes.
- **Partial success is easy to misread as full success** → the command
  prints one outcome line per node (deployed / guarded / failed) and exits
  non-zero on any failure, the same "row, not a silent gap" convention
  `fleet status` and `fleet metrics` already use for unreachable nodes.
- **A node's resolved Spinloop file drifts from its fleet-file entry
  unnoticed** → out of scope here; `fleet deploy`'s job is to run the deploy
  that file describes, not to detect drift. `spinloop fleet route` already
  gives an operator a way to check what a node is actually serving.
- **Three fallback tiers make it non-obvious which Spinloop file a node will
  actually use** → both `fleet deploy` and `fleet start` state the resolved
  source (the path used, or the alias name) before acting, the same way
  `remote deploy` already announces "Using alias …"; nothing happens
  silently from an unexpected source.
- **`fleet start` on a `kind: daemon` node with no resolvable source now
  fails instead of starting** → deliberate (see the "requires a daemon
  node's resolved source" decision); every fleet file with a `kind: daemon`
  node needs a `file` field, a matching alias, or a matching subdirectory
  before this ships, including the example fleets (task 7.3).
- **An alias or subdirectory coincidentally named after a node resolves to
  the wrong Spinloop** → mitigated by always announcing the resolved source
  before acting (previous bullet); an operator who wants a specific source
  can always pin it with an explicit `file` field, which wins over both
  fallbacks.

## Migration Plan

A new field and a new subcommand, plus a breaking change to `fleet start`
for any `kind: daemon` node with no resolvable Spinloop source (see
proposal.md, marked **BREAKING**) — every fleet file needs a `file` field,
alias, or subdirectory added for each `kind: daemon` node it lists,
including this repo's own example fleets (task 7.3). `remote deploy` itself
is unchanged. No data migration, no flag renames.
