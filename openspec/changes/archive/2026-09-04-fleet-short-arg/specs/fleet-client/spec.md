## MODIFIED Requirements

### Requirement: Fleet logs

`spinloop fleet logs` SHALL read the engine output of the fleet's nodes through
each node's daemon, so "what did that engine say?" is answerable from the same
place as "what is it doing?" — without shell access to any machine. With no node
named it SHALL read every node in the fleet; naming a node SHALL restrict it to
that one. Nodes SHALL be read concurrently, so the command's latency is that of
the slowest reachable node rather than their sum.

The fleet file SHALL be named by the long form `--fleet` only: unlike the other
`spinloop fleet` commands, `logs` SHALL NOT accept `-f` for it, because `-f` is
that command's `--follow` short form and a flag cannot carry two meanings on one
command line.

#### Scenario: Reading the whole fleet

- **WHEN** the operator runs `spinloop fleet logs` with no node named
- **THEN** every node's engine output is read and printed

#### Scenario: Reading one node

- **WHEN** the operator names a node
- **THEN** only that node's output is printed, and the other nodes are not
  contacted

#### Scenario: A crashed node's output is readable

- **WHEN** a node's engine has crashed, as `spinloop fleet status` reports
- **THEN** its output up to the crash is printed, explaining what status can
  only report

#### Scenario: The fleet file has no short flag here

- **WHEN** the operator runs `spinloop fleet logs -f --fleet ./cluster.yaml`
- **THEN** the flag is accepted as follow mode plus the fleet file, with `-f`
  not treated as a fleet-file flag
