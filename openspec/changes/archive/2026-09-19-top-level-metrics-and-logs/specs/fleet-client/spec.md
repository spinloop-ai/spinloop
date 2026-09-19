## MODIFIED Requirements

### Requirement: Fleet metrics

`spinloop metrics` SHALL query every node in the resolved target and render
each node's engine and system metrics using the same bar, gauge, table and
json formats, selected by `--format`.
Unreachable nodes SHALL be reported as in status rather than omitted. The
command SHALL support a `--watch`/`-w` mode that refreshes on an interval,
clearing and redrawing the screen in place with no scrollback accumulation,
and exiting cleanly on interrupt.

#### Scenario: Gauge format per node

- **WHEN** `spinloop metrics` runs without `--format`
- **THEN** each reachable node's resource series render in gauge format under
  its name

#### Scenario: Bar format per node

- **WHEN** `spinloop fleet metrics --format=bar` runs
- **THEN** each reachable node's metrics render in bar format under its name

#### Scenario: JSON aggregates the fleet

- **WHEN** `spinloop fleet metrics --format=json` runs
- **THEN** the output is valid JSON keyed or labelled by node, including
  unreachable nodes with their error

#### Scenario: Watch redraws in place

- **WHEN** `spinloop fleet metrics --watch` runs
- **THEN** each refresh clears the screen and redraws the fleet, and Ctrl+C
  exits cleanly

The target SHALL resolve as it does for every other command that acts on a
fleet — a named environment, a named fleet file, or the working directory's
fleet file — so one environment and a fleet holding it are read by the one
command.

#### Scenario: A named environment is read by the same command

- **WHEN** `spinloop metrics --env prod` runs
- **THEN** that environment's metrics are rendered with no fleet file
  required, in the same format a fleet file naming it would produce

#### Scenario: The fleet-scoped spelling names its replacement

- **WHEN** the operator runs `spinloop fleet metrics`
- **THEN** it fails naming `spinloop metrics` as the command that replaced it

### Requirement: Fleet logs

`spinloop logs` SHALL read the engine output of the target's nodes, so "what did that engine say?" is answerable from the same
place as "what is it doing?" — without shell access to any machine. With no node
named it SHALL read every node in the fleet; naming a node SHALL restrict it to
that one. Nodes SHALL be read concurrently, so the command's latency is that of
the slowest reachable node rather than their sum.

The target SHALL resolve as it does for every other command that acts on a
fleet. The fleet file SHALL be named by the long form `--fleet` only: unlike
the other commands that take one, `logs` SHALL NOT accept `-f` for it, because
`-f` is that command's `--follow` short form and a flag cannot carry two
meanings on one command line.

#### Scenario: Reading the whole fleet

- **WHEN** the operator runs `spinloop logs` with no node named
- **THEN** every node's engine output is read and printed

#### Scenario: Reading one node

- **WHEN** the operator names a node
- **THEN** only that node's output is printed, and the other nodes are not
  contacted

#### Scenario: A crashed node's output is readable

- **WHEN** a node's engine has crashed, as `spinloop status` reports
- **THEN** its output up to the crash is printed, explaining what status can
  only report

#### Scenario: The fleet file has no short flag here

- **WHEN** the operator runs `spinloop fleet logs -f --fleet ./cluster.yaml`
- **THEN** the flag is accepted as follow mode plus the fleet file, with `-f`
  not treated as a fleet-file flag

#### Scenario: A named environment is read by the same command

- **WHEN** `spinloop logs --env prod` runs
- **THEN** that environment's engine output is read with no fleet file
  required

#### Scenario: The fleet-scoped spelling names its replacement

- **WHEN** the operator runs `spinloop fleet logs`
- **THEN** it fails naming `spinloop logs` as the command that replaced it

## ADDED Requirements

### Requirement: A node answers only what its kind can

A node kind SHALL answer the operations its kind supports and no more. Where a
read verb offers a flag whose answer only some node kinds carry — an instance's
price, a log store's source, time window or instance id — the flag SHALL apply
to the nodes that can answer it and SHALL leave the rest as they read without
it.

A target holding no node that can answer SHALL NOT be an error. Rendering a
column blank for a node that has no such fact is what these views already do
for every fact only one kind reports, and a flag is not a different case: a
fleet of daemons asked for a cost is a fleet with no cost to show, not a
malformed command.

The capability SHALL be a property of the node kind, asserted by the caller,
not a field on the shared reply. A kind that cannot answer SHALL simply not
implement it, so adding a kind that can requires no change to the callers and
adding one that cannot requires no exception.

Which flags apply to which kinds SHALL be documented on each verb's page, since
a blank column is not self-explaining.

#### Scenario: A priced node and an unpriced one in one table

- **WHEN** `spinloop metrics --cost` runs against a target holding both a cloud
  environment and a daemon node
- **THEN** the environment's row carries its cost and the daemon's does not,
  and the command succeeds

#### Scenario: No node can answer, and that is not an error

- **WHEN** `spinloop metrics --cost` runs against a fleet of daemon nodes only
- **THEN** no cost is shown for any node and the command succeeds

#### Scenario: A log query narrows the nodes that support it

- **WHEN** `spinloop logs --source boot` runs against a target holding both a
  cloud environment and a daemon node
- **THEN** the environment's boot log is read, and the daemon's output is read
  as it would be without the flag

#### Scenario: A kind that cannot answer implements nothing

- **WHEN** a node kind has no answer for a capability
- **THEN** it does not implement that capability, and no caller carries a
  branch naming that kind
