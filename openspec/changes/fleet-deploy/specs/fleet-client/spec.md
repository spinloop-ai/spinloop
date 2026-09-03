## ADDED Requirements

### Requirement: Fleet deploy targets remote nodes

`spinloop fleet deploy [node...]` SHALL deploy the AWS environment for one or
more `kind: remote` nodes in the fleet file. Named with no arguments, it
SHALL target every `kind: remote` node in the file. Named with one or more
node names, it SHALL target exactly those. An unknown node name SHALL fail
the command, naming the known nodes, without deploying anything. A named
`kind: daemon` node SHALL fail the command, explaining that `fleet deploy`
provisions cloud environments and that node is not one; a `kind: daemon` node
present only because no nodes were named SHALL instead be skipped, reported
as skipped, and not counted as a failure.

#### Scenario: Deploy the whole remote fleet

- **WHEN** `spinloop fleet deploy` runs with no node arguments against a file
  mixing `kind: remote` and `kind: daemon` nodes
- **THEN** every `kind: remote` node is deployed, every `kind: daemon` node is
  reported as skipped, and the command's success does not depend on the
  skipped nodes

#### Scenario: Deploy named nodes

- **WHEN** `spinloop fleet deploy gpu-a gpu-b` runs and both are `kind:
  remote` nodes in the file
- **THEN** only those two are deployed, whatever else the file lists

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
source resolves to (see fleet-config's "Remote node deploy source" and
"...falls back to name-based lookup" requirements: its `file` field, else an
alias registered under its name, else a `<name>/` subdirectory beside the
fleet file), deriving the deploy config and registering the resulting
environment exactly as `spinloop remote deploy <file>` does for that same
file — the two SHALL NOT be able to disagree about what a given Spinloop file
deploys. A targeted node for which no source resolves SHALL fail for that
node alone, naming all three ways one could have been given, without
touching the other targeted nodes. The resolved source (the path used, or
the alias name when one was used) SHALL be reported alongside that node's
plan, so which of the three supplied it is never left to be inferred.

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

- **WHEN** `spinloop fleet deploy --dry-run` runs with no node arguments
- **THEN** the plan for every `kind: remote` node in the file is printed and
  no environment is created or registered
