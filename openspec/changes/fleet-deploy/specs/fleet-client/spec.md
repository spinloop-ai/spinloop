## ADDED Requirements

### Requirement: Fleet deploy targets remote nodes

`spinloop fleet deploy <node...>` SHALL deploy the AWS environment for one or
more `kind: remote` nodes in the fleet file, named explicitly.
`spinloop fleet deploy --all` SHALL target every `kind: remote` node in the
file instead. Invoked with neither a node name nor `--all`, it SHALL fail,
listing the fleet's `kind: remote` nodes, and deploy nothing — mutating
however many cloud environments a fleet file lists SHALL NOT happen by
default. `--all` combined with one or more node names SHALL fail as
ambiguous. An unknown node name SHALL fail the command, naming the known
nodes, without deploying anything. A named `kind: daemon` node SHALL fail
the command, explaining that `fleet deploy` provisions cloud environments
and that node is not one; `--all` SHALL only ever select `kind: remote`
nodes, so a `kind: daemon` node is never targeted by it and is not reported
at all.

#### Scenario: Deploy every remote node

- **WHEN** `spinloop fleet deploy --all` runs against a file mixing `kind:
  remote` and `kind: daemon` nodes
- **THEN** every `kind: remote` node is deployed and no `kind: daemon` node
  is touched or mentioned

#### Scenario: Deploy named nodes

- **WHEN** `spinloop fleet deploy gpu-a gpu-b` runs and both are `kind:
  remote` nodes in the file
- **THEN** only those two are deployed, whatever else the file lists

#### Scenario: No target is an error

- **WHEN** `spinloop fleet deploy` runs with no node arguments and no `--all`
- **THEN** it fails, listing the fleet's `kind: remote` nodes, and deploys
  nothing

#### Scenario: Combining --all with node names is an error

- **WHEN** `spinloop fleet deploy --all gpu-a` runs
- **THEN** it fails as ambiguous and deploys nothing

#### Scenario: An unknown node name fails the command

- **WHEN** `spinloop fleet deploy nope` runs and no node is named `nope`
- **THEN** the command fails, naming the known nodes, and deploys nothing

#### Scenario: Naming a daemon node explicitly fails

- **WHEN** `spinloop fleet deploy studio` runs and `studio` is a `kind:
  daemon` node
- **THEN** the command fails, explaining that `fleet deploy` provisions cloud
  environments and `studio` is not one

### Requirement: Fleet deploy derives and applies each node's config

Each targeted node SHALL be deployed from the Spinloop file its deploy
source resolves to (see fleet-config's "Node Spinloop source" and "...falls
back to name-based lookup" requirements: its `file` field, else an alias
registered under its name, else a `<name>/` subdirectory beside the fleet
file), deriving the deploy config and registering the resulting environment
exactly as `spinloop remote deploy <file>` does for that same file — the two
SHALL NOT be able to disagree about what a given Spinloop file deploys. A
targeted node for which no source resolves SHALL fail for that node alone,
naming all three ways one could have been given, without touching the other
targeted nodes. The resolved source (the path used, or the alias name when
one was used) SHALL be reported alongside that node's plan, so which of the
three supplied it is never left to be inferred.

Nodes SHALL be deployed independently: one node already registered or live
SHALL require `--overwrite` for that node exactly as a standalone `remote
deploy` does, and refusing it SHALL NOT stop the other targeted nodes from
deploying. A node whose deploy fails for any other reason SHALL likewise be
reported against that node without aborting the rest. The command SHALL exit
non-zero when any targeted node failed to deploy, having still attempted
every other targeted node.

`--dry-run` SHALL print the plan for every targeted node without deploying
any of them, exactly as a standalone `remote deploy --dry-run` does for one.
`--overwrite` SHALL apply to every targeted node that needs it.

#### Scenario: A node deploys from its own Spinloop file

- **WHEN** `fleet deploy` targets a node declaring `file:
  ./envs/gpu.Spinloop`
- **THEN** that node's environment is created and registered from that file,
  the same as `spinloop remote deploy ./envs/gpu.Spinloop` would produce, and
  the resolved path is reported against that node

#### Scenario: A node with no resolvable source fails only that node

- **WHEN** `fleet deploy` targets two remote nodes and one declares no `file`
  field, has no alias registered under its name, and has no same-named
  subdirectory beside the fleet file
- **THEN** the other node still deploys, and the command reports against the
  unresolved node that none of the `file` field, a matching alias, or a
  matching subdirectory was found

#### Scenario: One node's guard does not block the others

- **WHEN** `fleet deploy` targets two remote nodes and one is already
  registered while the other is not, and `--overwrite` is not given
- **THEN** the unregistered node deploys, the registered node is refused with
  the same message a standalone `remote deploy` gives, and the command exits
  non-zero

#### Scenario: Dry run previews every targeted node

- **WHEN** `spinloop fleet deploy --dry-run --all` runs
- **THEN** the plan for every `kind: remote` node in the file is printed and
  no environment is created or registered

## MODIFIED Requirements

### Requirement: Driving one node

`spinloop fleet stop <node>` SHALL call the named node's daemon stop
endpoint. Stop SHALL require exactly one node name: invoked without one it
SHALL fail and list the available nodes, rather than acting on the whole
fleet, and it SHALL NOT accept `--all` or more than one name. An unknown
node name SHALL fail, naming the known nodes. A stop is idempotent.

`spinloop fleet start <node...>` SHALL call each named node's daemon start
endpoint (or push a resolved deploy config, for a `kind: daemon` node — see
below). `spinloop fleet start --all` SHALL target every node in the file
instead, of either kind. Start invoked with neither a node name nor `--all`
SHALL fail and list the available nodes. `--all` combined with one or more
node names SHALL fail as ambiguous. An unknown node name SHALL fail the
command, naming the known nodes, before anything is started. The daemon's
own rules still hold — a start while that node's engine is running is
reported as the daemon's conflict for that node. Multiple targeted nodes
SHALL start independently: one node's failure (including an unresolved
Spinloop source, see below) SHALL be reported against that node alone and
SHALL NOT stop the others; the command SHALL exit non-zero when any targeted
node failed.

For a `kind: daemon` node, `fleet start` SHALL first resolve that node's
Spinloop source (see fleet-config's "Node Spinloop source" and "...falls
back to name-based lookup" requirements), and SHALL fail that node's start,
naming all three ways a source could have been given, when none resolves.
When one resolves, the client SHALL derive a deploy config from it — the
same node-owned derivation a routed wake already uses
(`deployConfigForNode`) — report the resolved source and derived config
alongside the node's name, and start the node's engine with that config
(`StartWith`) rather than a plain start, exactly as a routed wake tells a
node what to serve. A `kind: remote` node's start is unaffected regardless
of whether a source resolves for it: what it serves is fixed at deploy time,
not pushed at start time, so it always uses a plain start.

#### Scenario: Start a named node

- **WHEN** `spinloop fleet start gpu-box` runs, that node is idle, and its
  Spinloop source resolves
- **THEN** the client derives a deploy config from the resolved source and
  calls that node's daemon start endpoint with it, reporting the resulting
  state

#### Scenario: Start several named nodes

- **WHEN** `spinloop fleet start gpu-a gpu-b` runs and both nodes' Spinloop
  sources resolve
- **THEN** both nodes start, independently, whatever else the file lists

#### Scenario: Start every node

- **WHEN** `spinloop fleet start --all` runs against a file mixing `kind:
  remote` and `kind: daemon` nodes, and every `kind: daemon` node's Spinloop
  source resolves
- **THEN** every node in the file starts — the daemon nodes with their
  resolved config, the remote nodes with a plain start

#### Scenario: Start with no node names the fleet

- **WHEN** `spinloop fleet start` runs with no node argument and no `--all`
- **THEN** it fails, listing the nodes, and starts nothing

#### Scenario: Combining --all with node names is an error

- **WHEN** `spinloop fleet start --all gpu-a` runs
- **THEN** it fails as ambiguous and starts nothing

#### Scenario: Unknown node

- **WHEN** `spinloop fleet stop nope` runs and no node is named `nope`
- **THEN** it fails, naming the known nodes, and stops nothing

#### Scenario: An unknown name among several fails before starting any

- **WHEN** `spinloop fleet start gpu-a nope` runs and no node is named `nope`
- **THEN** the command fails, naming the known nodes, and starts neither node

#### Scenario: Starting a daemon node with a resolved source pushes it

- **WHEN** `spinloop fleet start dev-1` runs, `dev-1` is a `kind: daemon`
  node, and its Spinloop source resolves (by `file`, alias, or subdirectory)
- **THEN** the client derives a deploy config from the resolved Spinloop,
  reports the resolved source, and starts `dev-1`'s engine with that config

#### Scenario: Starting a daemon node with no resolvable source fails

- **WHEN** `spinloop fleet start studio` runs, `studio` is a `kind: daemon`
  node, and no `file` field, alias, or subdirectory resolves for it
- **THEN** the command fails for `studio`, naming the `file` field, the
  alias registry, and the subdirectory convention as the three ways a source
  could have been given, and nothing is started

#### Scenario: One unresolved node among several fails only that node

- **WHEN** `spinloop fleet start --all` runs, and one `kind: daemon` node in
  the file has no resolvable Spinloop source while the rest do
- **THEN** every other targeted node starts, the unresolved node is reported
  as failed naming the three ways a source could have been given, and the
  command exits non-zero

#### Scenario: Starting a remote node is unaffected by a resolved source

- **WHEN** `spinloop fleet start gpu-env` runs, `gpu-env` is a `kind: remote`
  node, and a Spinloop source resolves for it
- **THEN** the client starts it with a plain start; the resolved source is
  not used, since a `kind: remote` node's `StartWith` always refuses a
  deploy config
