# fleet-gateway Specification

## ADDED Requirements

### Requirement: Serving the fleet's topology

The gateway SHALL serve the fleet's topology over a read endpoint, behind the
same caller authentication as everything else it serves: a caller with the
token gets it, a caller without does not, exactly as on its other paths.

The reply SHALL describe each node of the fleet the gateway holds: its name,
its kind, its tags as the fleet file declares them, its state, and its
serving facts — the model it serves when it is running, the name it serves
that model under where it reports one, whether its engine has answered, and
when it was last active. For a node that is not running, the reply SHALL name
the model a request would start it with, where the node's own source
describes one and the fleet's wake policy allows starting it; a node whose
source describes no such model SHALL report none. A node that does not answer
SHALL be reported as such in the fleet's order, not fail the whole reply.

The reply SHALL carry the fleet file's fleet-level settings the way the file
declares them: whether the fleet wakes, how it ranks, and its concurrency
limits where it declares any, with each absent where the file declares it
not. The gateway's fleet file remains the single source of truth for the
topology: the reply is what the file says and what the nodes report, and the
gateway holds no copy of either beyond what it already holds.

#### Scenario: A caller with the token reads the topology

- **WHEN** a caller sends the gateway's token to the topology endpoint
- **THEN** it gets every node with its tags, state, and serving facts, and
  the file's wake policy, ranking, and concurrency limits

#### Scenario: A caller without the token is refused

- **WHEN** a caller sends no token, or the wrong one, to the topology
  endpoint
- **THEN** it is refused the way the gateway's other paths refuse it

#### Scenario: A stopped node reports what it would start

- **WHEN** a node is not running, its own source describes a model, and the
  fleet wakes
- **THEN** the topology names that model as what a request would start the
  node with

#### Scenario: A dead node does not sink the reply

- **WHEN** one of the fleet's nodes does not answer and a caller reads the
  topology
- **THEN** that node is reported as not answering, in the fleet's order, and
  the rest of the fleet is reported as usual

#### Scenario: Absent settings are absent

- **WHEN** the fleet file declares no concurrency limits
- **THEN** the topology carries none, rather than a default the file never
  named
