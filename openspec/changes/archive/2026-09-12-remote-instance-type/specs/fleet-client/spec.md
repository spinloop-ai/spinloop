## MODIFIED Requirements

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

Where a node declares an `instance-type` in the fleet file, the deploy config
derived for it SHALL carry that type, so the node's environment launches as
named — the same value a standalone `spinloop remote deploy --instance-type`
would record for the environment — and a node declaring none SHALL deploy an
environment on the control plane's default type. This keeps `fleet deploy` and
a matching standalone deploy in agreement about what a node's environment
launches as.

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

#### Scenario: A node's declared instance type is deployed

- **WHEN** `fleet deploy` targets a `kind: remote` node declaring
  `instance-type: g6e.2xlarge`
- **THEN** the environment it deploys launches as `g6e.2xlarge`, the same
  value a standalone `spinloop remote deploy --instance-type g6e.2xlarge` of
  the node's source would record

#### Scenario: A node with no instance type deploys the default

- **WHEN** `fleet deploy` targets a `kind: remote` node declaring no
  `instance-type`
- **THEN** the environment it deploys launches as the control plane's default
  instance type

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
