## MODIFIED Requirements

### Requirement: The fleet file declares remote nodes

The fleet file SHALL be able to list a remote environment as one of its nodes, alongside
daemon nodes. The node's kind SHALL default to daemon. A node of kind `remote` SHALL be
keyed by its name: the name IS the registered environment it drives (there is no separate
address field, because an environment is already user-named at deployment), and such a
node SHALL need no host, because the environment's control URLs come from that
environment's own config rather than the fleet file. Because the name doubles as the
environment key, it SHALL be constrained to an environment shape — no path separator, no
`.json` suffix — so a path-like name is rejected rather than read as a registry directory.
Building the live node for a `remote` entry SHALL load that environment's config keyed by
its name, and an environment that is not registered SHALL fail as a per-node
configuration error — naming the environment — rather than failing the command or blanking
the view. A fleet of remote environments, or of daemons and remote environments mixed,
SHALL be observable and drivable (status, metrics, start, stop) through the same fan-out
as a fleet of daemons alone.

A node of kind `remote` MAY declare an optional `instance-type` naming the EC2 instance
type its environment launches as. It is a property of the remote environment only: a
`kind: daemon` node naming one SHALL be rejected, because a daemon's hardware is the
operator's to choose, not something the fleet file provisions. When present, the value
SHALL be checked for the shape of an EC2 instance type — a lowercase family and size
separated by a single dot, as in `g6e.xlarge` — when the file is read, so a typo is named
at parse rather than at launch. A `kind: remote` node naming no `instance-type` SHALL
deploy an environment that launches as the control plane's default, unchanged from before
the field existed.

#### Scenario: A fleet file lists a remote environment as a node

- **WHEN** a fleet file lists a node of kind `remote` whose name is a registered
  environment
- **THEN** the fan-out builds it as a remote node and observes it as one row, alongside any
  daemon nodes in the same file

#### Scenario: A fleet file lists a remote without its environment

- **WHEN** a fleet file lists a node of kind `remote` whose name is not a registered
  environment
- **THEN** that node is reported with a configuration error naming the environment, and
  the rest of the fleet is still observed

#### Scenario: A remote node's name must be env-shaped

- **WHEN** a fleet file lists a node of kind `remote` whose name contains a path separator
  or a `.json` suffix
- **THEN** the fleet file is rejected, naming the node, because the name is the environment
  key

#### Scenario: A remote node names its instance type

- **WHEN** a fleet file lists a `kind: remote` node declaring `instance-type: g6e.2xlarge`
- **THEN** the file parses, and that node's environment is deployed to launch as
  `g6e.2xlarge`

#### Scenario: A daemon node naming an instance type is rejected

- **WHEN** a fleet file lists a `kind: daemon` node declaring an `instance-type`
- **THEN** the file is rejected, naming the node, because instance type is a property of
  a remote environment only

#### Scenario: A malformed instance type is named at parse

- **WHEN** a fleet file lists a `kind: remote` node whose `instance-type` is not shaped
  like an EC2 instance type
- **THEN** the file is rejected, naming the node and the value
