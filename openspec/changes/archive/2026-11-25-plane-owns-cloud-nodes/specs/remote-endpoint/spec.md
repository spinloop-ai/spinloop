# Spec Delta

## REMOVED Requirements

### Requirement: Environment selection for cloud commands

**Reason**: The requirement selected an environment by reading its
`cloud.json` from the per-user registry, and its scenarios both demanded and
forbade a `default` fallback. The control URLs now come from the plane the
caller's credentials and region resolve, and there is definitively no fallback.
The replacement requirement is `Nodes are selected by name on the resolved
plane`.

**Migration**: `--node <name>` is unchanged at the keyboard; what changes is
where the name is looked up. Scripts that relied on an assumed `default`
name must now name the node.

## ADDED Requirements

### Requirement: Nodes are selected by name on the resolved plane

The endpoint's control URLs SHALL come from the control plane the caller's
credentials and region resolve, read from its stack outputs, plus the
`SPINLOOP_CLOUD_*` overrides where those state values directly; they SHALL NOT
come from a stored configuration file. A resolved plane names a start URL, a
stop URL, and the other control endpoints, and a region. The endpoint's own base
URL SHALL NOT need to be held anywhere: the control calls report the address
themselves.

A `cloud` subcommand SHALL select which node it acts on with its
`--node <name>` flag, and the flag SHALL be required: the value is a
node name on the resolved plane. A `--node` value naming a node
the plane does not hold, and which the variables do not configure, SHALL fail
saying the plane holds no such node and how to create it, naming the
account and region resolved.

A subcommand given no `--node` SHALL fail naming the flag and listing the
plane's nodes, and SHALL act on nothing. There is no node a
command falls back to: several of these subcommands change the state of a cloud
instance, an instance nobody named is not one to start, stop or terminate, and
on a shared plane an assumed name could act on a colleague's deployment. The
name `default` is an ordinary node name, carrying no special meaning.

The Spinloop a subcommand is given as an argument SHALL NOT select a
node; it SHALL be read only for its `ENV` instructions and the `.env`
file beside it, which the command applies before any AWS or control-plane work
(see the Cloud Local Node specification).

Node variables SHALL override individual values, and the region SHALL
fall back to the standard AWS region variables. A plane that cannot be resolved
SHALL fail naming where it looked.

#### Scenario: The flag selects the node

- **WHEN** the user runs `spinloop cloud status --node qwen3.6-27b-prod`
- **THEN** the control URLs come from the resolved plane and the node is
  the one named, wherever it was deployed from

#### Scenario: A node the plane lacks is named as such

- **WHEN** a `cloud` subcommand runs with `--node missing` and the resolved
  plane holds no node `missing`
- **THEN** it fails saying the plane holds no such node, naming the
  account and region resolved and that `spinloop cloud deploy --node missing`
  creates it

#### Scenario: No flag acts on nothing

- **WHEN** a `cloud` subcommand runs with no `--node` flag
- **THEN** it fails naming the flag and listing the plane's nodes, and
  contacts no node

#### Scenario: An explicit Spinloop does not select a node

- **WHEN** a `cloud` subcommand is given a Spinloop as its argument and no
  `--node` flag
- **THEN** the command fails naming `--node`, having applied the Spinloop's
  `ENV` instructions and adjacent `.env` to the process environment, rather than
  assuming a node for it

#### Scenario: No address needs to be held anywhere

- **WHEN** a `cloud` subcommand runs against a node whose address has
  changed since it was deployed, and nothing local names any address
- **THEN** the subcommand works, because the endpoint reports its own address in
  the replies to `start` and `status`

#### Scenario: An instance is never stopped without being named

- **WHEN** the user runs `spinloop cloud stop` with no `--node`, with a
  node named `default` deployed on the plane
- **THEN** nothing is stopped: the command fails naming the flag, and `default`
  is not assumed

#### Scenario: default is an ordinary name

- **WHEN** the user runs a `cloud` subcommand with `--node default` and the plane
  holds a node `default`
- **THEN** it acts on that node, exactly as it would for any other name
