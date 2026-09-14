## MODIFIED Requirements

### Requirement: A remote environment is a fleet node

A registered remote environment SHALL be representable as one member of the fleet's node
set, answering the same operations a local node answers: its status, its metrics, and
being started, stopped, and read for logs. The control plane's replies SHALL be mapped
onto the same status and metrics shapes a local node yields, so downstream fan-out and
rendering treat the two identically. A running environment's status SHALL in particular
carry what its engine is serving — the model it runs, and the served name the deploy gave
it beside the model id when there is one — so a client choosing a node by model matches
it the way it matches a local node, and the fleet view and the remote view name it the
same. It SHALL also carry where its engine answers — the instance's published address,
which the control plane knows and a daemon on the instance cannot — so a client can
reach the engine, not only name it; a stopped or undeployed environment reports none.

A remote environment that cannot be reached, or whose control call is rejected —
including a rejected AWS credential — SHALL be reported as a typed outcome against that
environment, the same way an unreachable or unauthorized node is, rather than failing the
command or being silently dropped.

A deployed remote environment — one whose stored deploy config already describes what to
serve — SHALL answer a node-level start the same way a local node does: the instance is
booted and the call waits for it, without deploying a new configuration. Any deploy
configuration a caller supplies to the start SHALL NOT be pushed onto the environment: it
already knows what to run, and choosing what it runs is `spinloop remote deploy`'s job, not
a node start's. An undeployed remote environment — one with no stored deploy config —
SHALL still refuse a node-level start, with a message naming the deployment path, rather
than attempted: starting one would mean choosing what to serve and paying for provisioning
and weights, a heavier decision a node start must not make on a caller's behalf.

#### Scenario: A remote environment answers status like a node

- **WHEN** a remote environment is asked for its status as a member of a node set
- **THEN** it returns a status carrying the endpoint's state, what its engine is serving
  (the model, and the served name beside it when the deploy gave one), where its engine
  answers (the instance's published address), and, when the engine has done work, its
  last-active time, in the same shape a local node's status carries

#### Scenario: A freshly loaded engine shows its model before it has done work

- **WHEN** a remote environment's engine is serving a model but has not yet answered a
  request, so it reports no last-active time
- **THEN** its status still carries the model it is serving, so a router can match a
  request to it before the first request has landed

#### Scenario: A remote environment answers metrics like a node

- **WHEN** a running remote environment is asked for its metrics as a member of a node set
- **THEN** it returns the token and system figures in the same stats shape a local node
  returns

#### Scenario: A rejected control call is a typed outcome

- **WHEN** a remote environment's status or metrics call is rejected, for example because
  the caller's credentials are not valid
- **THEN** the environment is reported with a failure outcome and the reason, and it does
  not abort or blank the rest of the node set

#### Scenario: A deployed remote environment is started

- **WHEN** a node-level start is requested for a stopped remote environment whose stored
  deploy config already describes what to serve
- **THEN** the environment's instance is booted, the call waits for it the way a local
  node's start does, and the environment serves what its own stored config names, not
  any config the start call carried

#### Scenario: Waking a remote environment is refused

- **WHEN** a node-level start is requested for a remote environment with no stored deploy
  config
- **THEN** it is refused with a message naming the deployment path, and the environment
  is not started
