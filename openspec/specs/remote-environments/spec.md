# Cloud Nodes Specification

## Purpose

Define how cloud nodes are named and resolved on the control plane: the
shape a node name must have, how a `--node` value resolves to a node on
the plane the caller's credentials reach, how the plane's nodes are
listed, and how a node keys a harness provider. The plane — not any
machine's local state — is the record of what is deployed.

## Requirements

### Requirement: Node name validity

A node name SHALL be a plain name, not a path: it SHALL NOT contain a
path separator, and it SHALL satisfy the shape the control plane accepts for a
node name. A `--node` value that is not a plain identifier SHALL be
rejected saying a node name is a plain identifier.

Because a name identifies one node on the plane the caller's credentials
resolve, the same name on two planes SHALL be two nodes, and the same
name on one plane SHALL be one node however many people use it.

#### Scenario: A path-like value is not a name

- **WHEN** a command is given `--node ./cloud.json` (or any value containing a
  path separator)
- **THEN** it fails saying a node name is a plain identifier, rather
  than reading a file or querying a plane

#### Scenario: A name the control plane would refuse is refused early

- **WHEN** a command is given a name the control plane would reject
- **THEN** it fails before contacting the plane, naming the shape a name must
  have

#### Scenario: The same name on two planes is two nodes

- **WHEN** two operators each run `--node dev` with credentials for different
  accounts
- **THEN** each acts on the `dev` deployed in their own account

### Requirement: Listing nodes

`spinloop cloud ls` SHALL print every node deployed on the resolved
plane with its deployer, what it serves, its address and its state, rather than
what this machine has recorded. With no nodes on the plane it SHALL say
so plainly rather than printing nothing. Where the plane cannot be resolved or
reached it SHALL fail naming where it looked rather than printing a partial
list.

#### Scenario: Listing shows each node

- **WHEN** two nodes are deployed on the plane and the user runs
  `spinloop cloud ls`
- **THEN** both are listed with their deployer, what they serve, their address
  and their state

#### Scenario: A node deployed elsewhere is listed

- **WHEN** a node deployed by another operator is on the plane and the
  user runs `spinloop cloud ls`
- **THEN** it is listed like any other, credited to its deployer

#### Scenario: A missing configuration is marked, not fatal

- **WHEN** a node is deployed but the plane cannot report its state or
  address — its instance was terminated mid-read, or its daemon is unreachable
- **THEN** that node is listed with a marker where the readings are
  missing, and the command still succeeds

#### Scenario: No nodes on the plane

- **WHEN** the plane holds no nodes and the user runs `spinloop cloud ls`
- **THEN** the command says there are none, rather than printing empty output

#### Scenario: An unreadable plane is not an empty one

- **WHEN** `spinloop cloud ls` runs where the plane cannot be reached
- **THEN** the command fails naming where it looked, and prints no list

### Requirement: A node names the harness provider

When a Spinloop is applied with an `--node <name>` flag, the harness provider
SHALL be keyed on the node name rather than the `PROVIDER` value, and
the default model SHALL read as `<node>/<model>`. The `PROVIDER` entry
SHALL still supply the engine configuration (its options, API-key node
variable, and base URL). The flag SHALL name a node deployed on the
resolved plane: a value with no such node SHALL fail saying the plane
holds no node of that name and that `spinloop cloud deploy --node <name>`
creates one. A Spinloop's own `BASEURL` SHALL still supply the applied base
URL, as it does when a fleet is named; where the Spinloop states none, the base
URL SHALL be the node's own address as the plane reports it, without
starting the node. Unapplying with the same `--node` flag SHALL remove
the provider that apply wrote.

#### Scenario: The flag becomes the provider name

- **WHEN** a Spinloop stating `PROVIDER llamacpp` and `ALIAS qwen` is applied
  with `--node dev-1`
- **THEN** the harness config holds a provider keyed `dev-1` whose default
  model is `dev-1/qwen`, configured from the `llamacpp` catalogue entry

#### Scenario: A pinned BASEURL supplies the address

- **WHEN** a Spinloop stating a `BASEURL` is applied with `--node dev-1` whose
  node answers at a different address
- **THEN** the provider is keyed `dev-1` and configured with the Spinloop's
  `BASEURL`

#### Scenario: No BASEURL takes the node's address from the plane

- **WHEN** a Spinloop stating no `BASEURL` is applied with `--node dev-1`
- **THEN** the provider is configured with the address the plane reports for
  `dev-1`, and the node is not started to learn it

#### Scenario: A node the plane lacks is reported

- **WHEN** a Spinloop is applied with `--node missing` and the plane holds no
  node `missing`
- **THEN** the command fails saying the plane holds no such node and that
  `spinloop cloud deploy --node missing` creates it

#### Scenario: Unapply removes the node-named provider

- **WHEN** a Spinloop is applied with `--node dev-1` and then unapplied with
  `--node dev-1`
- **THEN** the provider keyed `dev-1` is removed from the harness config

### Requirement: Cloud provider display name

When a Spinloop is applied with an `--node` flag, the harness provider's display
name SHALL be distinct from that of the local engine of the same kind, so the
cloud environment and a local provider built from the same `PROVIDER` entry
are told apart in a harness model picker. The display name SHALL combine the
catalogue engine's display name with the environment name named by the flag
(for example `llama.cpp (dev-2)`); when the catalogue entry has no display
name, the environment name SHALL be used on its own. This labelling SHALL apply
only to the display name — the provider key, the `<environment>/<model>`
default model, the engine options, the API-key environment variable, and the
base URL are unchanged from the existing cloud-naming behaviour.

#### Scenario: A cloud provider is labelled with its environment

- **WHEN** a Spinloop stating `PROVIDER llamacpp` (display name `llama.cpp`)
  and `ALIAS qwen` is applied with `--node dev-2`
- **THEN** the harness config holds a provider keyed `dev-2` whose display name
  is `llama.cpp (dev-2)`, distinct from a local `llamacpp` provider's
  `llama.cpp`

#### Scenario: A local and a cloud engine of the same kind are distinguishable

- **WHEN** both a local `llamacpp` provider and a cloud `dev-2` provider built
  from the same engine are present in the harness config
- **THEN** their display names differ, so the two appear as separate rows in
  the harness model picker rather than two identical `llama.cpp` rows

#### Scenario: An engine with no display name falls back to the environment name

- **WHEN** a Spinloop's `PROVIDER` catalogue entry has no display name and the
  Spinloop is applied with `--node dev-2`
- **THEN** the harness provider's display name is `dev-2`

### Requirement: A node resolves by name on the plane

A node SHALL be resolved by its name and nothing else: the node
of that name on the plane the caller's credentials and region resolve. No name
SHALL resolve by a different rule from any other, and no local file SHALL be
consulted for any name, so a command that resolves a node never
special-cases the name it was given and never depends on what this machine has
deployed.

Where the caller's variables supply a complete set of control endpoints, those
SHALL be the plane, so the cloud commands work in a place with no
CloudFormation-visible stack. In that case the **name given** SHALL be the
node identifier sent with each control call — the identifier the shared
lifecycle endpoints use to tell one instance from another.

#### Scenario: A named node is found on the plane

- **WHEN** a command runs with `--node prod` and the resolved plane holds a
  node `prod`
- **THEN** the command drives that node, whether or not this machine ever
  deployed it

#### Scenario: Nothing local is consulted

- **WHEN** a command runs with `--node prod` and a file for a node
  `prod` exists under the configuration directory from an earlier version
- **THEN** the command uses the plane's answer and does not read that file

#### Scenario: Overrides configure a plane with no stack

- **WHEN** a command runs with `--node ci`, no control-plane stack is visible,
  and the `SPINLOOP_CLOUD_*` variables supply a complete set of control
  endpoints
- **THEN** the command works, and the control calls carry `ci` as the
  node identifier

#### Scenario: A node the plane does not hold fails naming the plane

- **WHEN** no node of the given name is deployed on the plane that
  resolves
- **THEN** the failure says the plane holds no such node, names the
  account and region resolved, and says which command creates one
