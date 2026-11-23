# Spec Delta

## RENAMED Requirements

- FROM: `### Requirement: A cloud environment is a fleet node`
- TO: `### Requirement: A cloud node is a fleet node`
## MODIFIED Requirements

### Requirement: A cloud node is a fleet node

A cloud node deployed on the plane the caller's credentials and region
resolve SHALL be representable as one member of the fleet's node set, answering
the same operations a local node answers: its status, its metrics, and being
started, stopped, and read for logs. The control plane's replies SHALL be mapped
onto the same status and metrics shapes a local node yields, so downstream
fan-out and rendering treat the two identically. A running node's status
SHALL in particular carry what its engine is serving — the model it runs, and
the served name the deploy gave it beside the model id when there is one — so a
client choosing a node by model matches it the way it matches a local node, and
the fleet view and the cloud view name it the same. It SHALL also carry where
its engine answers — the instance's published address, which the control plane
knows and a daemon on the instance cannot — so a client can reach the engine,
not only name it; a stopped or undeployed node reports none.

Whether a node exists at all SHALL be settled by the plane — its
inventory of deployed nodes — rather than inferred from a control call
that failed for another reason. A cloud node that cannot be reached, or
whose control call is rejected — including a rejected AWS credential — SHALL be
reported as a typed outcome against that node, the same way an
unreachable or unauthorized node is, rather than failing the command or being
silently dropped.

A node deployed on the plane — one whose stored deploy config already
describes what to serve — SHALL answer a node-level start the same way a local
node does: the instance is booted and the call waits for it, without deploying a
new configuration. Any deploy configuration a caller supplies to the start SHALL
NOT be pushed onto the node: it already knows what to run, and choosing
what it runs is `spinloop cloud deploy`'s job, not a node start's. An
node the plane does not hold — one with no deploy config stored there —
SHALL still refuse a node-level start, with a message naming the deployment
path, rather than attempted: starting one would mean choosing what to serve and
paying for provisioning and weights, a heavier decision a node start must not
make on a caller's behalf.

#### Scenario: A cloud environment answers status like a node

- **WHEN** a cloud node is asked for its status as a member of a node set
- **THEN** it returns a status carrying the endpoint's state, what its engine is
  serving (the model, and the served name beside it when the deploy gave one),
  where its engine answers (the instance's published address), and, when the
  engine has done work, its last-active time, in the same shape a local node's
  status carries

#### Scenario: A freshly loaded engine shows its model before it has done work

- **WHEN** a cloud node's engine is serving a model but has not yet
  answered a request, so it reports no last-active time
- **THEN** its status still carries the model it is serving, so a router can
  match a request to it before the first request has landed

#### Scenario: A cloud environment answers metrics like a node

- **WHEN** a running cloud node is asked for its metrics as a member of a
  node set
- **THEN** it returns the token and system figures in the same stats shape a
  local node returns

#### Scenario: A rejected control call is a typed outcome

- **WHEN** a cloud node's status or metrics call is rejected, for example
  because the caller's credentials are not valid
- **THEN** the node is reported with a failure outcome and the reason, and
  it does not abort or blank the rest of the node set

#### Scenario: A deployed cloud environment is started

- **WHEN** a node-level start is requested for a stopped cloud node whose
  stored deploy config already describes what to serve
- **THEN** the node's instance is booted, the call waits for it the way a
  local node's start does, and the node serves what its own stored config
  names, not any config the start call carried

#### Scenario: Waking a cloud environment is refused

- **WHEN** a node-level start is requested for a cloud node with no
  deploy config stored on the plane
- **THEN** it is refused with a message naming the deployment path, and the
  node is not started

#### Scenario: A node deployed by someone else is a node like any other

- **WHEN** a fleet node names a node deployed by another operator on the
  resolved plane
- **THEN** it is observed and driven exactly as one deployed on this machine
  would have been, with no local record consulted

### Requirement: The fleet file declares cloud nodes

The fleet file SHALL be able to list a cloud node as one of its nodes,
alongside daemon nodes. The node's kind SHALL default to daemon. A node of kind
`cloud` SHALL be keyed by its name: the name IS the node on the plane the
caller's credentials and region resolve (there is no separate address field,
because a node is named at deployment and the plane is resolved from the
credential), and such a node SHALL need no host, because the node's
control URLs come from the resolved plane rather than from the fleet file.
Because no deployment address, account or identity appears in the file, the same
file SHALL serve every operator with credentials for that plane, and it SHALL be
safe to commit. Because the name doubles as the node key, it SHALL be
constrained to a node shape — no path separator, no `.json` suffix — so a
path-like name is rejected rather than mistaken for a path. Building the live
node for a `cloud` entry SHALL resolve that node by name on the resolved
plane, and a node the plane does not hold SHALL fail as a per-node
configuration error — naming the node and the plane it was looked for on
— rather than failing the command or blanking the view. A fleet of cloud
nodes, or of daemons and cloud nodes mixed, SHALL be observable
and drivable (status, metrics, start, stop) through the same fan-out as a fleet
of daemons alone.

A node of kind `cloud` MAY declare an optional `instance-type` naming the EC2
instance type its node launches as. It is a property of the cloud
node only: a `kind: daemon` node naming one SHALL be rejected, because a
daemon's hardware is the operator's to choose, not something the fleet file
provisions. When present, the value SHALL be checked for the shape of an EC2
instance type — a lowercase family and size separated by a single dot, as in
`g6e.xlarge` — when the file is read, so a typo is named at parse rather than at
launch. A `kind: cloud` node naming no `instance-type` SHALL deploy a
node that launches as the control plane's default, unchanged from before
the field existed.

#### Scenario: A fleet file lists a cloud environment as a node

- **WHEN** a fleet file lists a node of kind `cloud` whose name is a node
  on the resolved plane
- **THEN** the fan-out builds it as a cloud node and observes it as one row,
  alongside any daemon nodes in the same file

#### Scenario: A fleet file lists a cloud node without its environment

- **WHEN** a fleet file lists a node of kind `cloud` whose name is not an
  node on the resolved plane
- **THEN** that node is reported with a configuration error naming the
  node and the plane it was looked for on, and the rest of the fleet is
  still observed

#### Scenario: The same file serves a colleague

- **WHEN** a committed fleet file listing cloud nodes is run by a second operator
  with credentials for the same plane and no local cloud configuration of their
  own
- **THEN** every cloud node resolves and is observed, with nothing for them to
  register or redeploy first

#### Scenario: A cloud node's name must be env-shaped

- **WHEN** a fleet file lists a node of kind `cloud` whose name contains a path
  separator or a `.json` suffix
- **THEN** the fleet file is rejected, naming the node, because the name is the
  node key

#### Scenario: A cloud node names its instance type

- **WHEN** a fleet file lists a `kind: cloud` node declaring
  `instance-type: g6e.2xlarge`
- **THEN** the file parses, and that node's node is deployed to launch as
  `g6e.2xlarge`

#### Scenario: A daemon node naming an instance type is rejected

- **WHEN** a fleet file lists a `kind: daemon` node declaring an `instance-type`
- **THEN** the file is rejected, naming the node, because instance type is a
  property of a cloud node only

#### Scenario: A malformed instance type is named at parse

- **WHEN** a fleet file lists a `kind: cloud` node whose `instance-type` is not
  shaped like an EC2 instance type
- **THEN** the file is rejected, naming the node and the value
