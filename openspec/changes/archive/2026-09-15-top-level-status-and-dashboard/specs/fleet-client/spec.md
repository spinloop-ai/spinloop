## MODIFIED Requirements

### Requirement: Fleet status

`spinloop status` SHALL query every node in the resolved target and render one row per node: the node name, its engine state (`idle`/`running`/`stopped`/`crashed`), what it is serving (runner and model when known), the spinloop version of the daemon on that node, and its reachability. Nodes SHALL be queried concurrently so the command's latency is that of the slowest reachable node, not their sum.

The target SHALL resolve as it does for every other command that acts on a fleet — a named environment, a named fleet file, or the working directory's fleet file — so one environment, a fleet, and a fleet holding that same environment are all read by the one command, rendered identically however the target was named.

A node's health SHALL be reported as the readiness mark every node carries, so a cloud environment whose endpoint is unhealthy reads the same as a daemon node whose engine has not answered its health check. A field only one node kind fills — an endpoint's address, a retention deadline — SHALL NOT be a column of a table meant to be scanned one row per node.

A node SHALL also report how long it has been since its engine last did work, taken from the activity its daemon tracks — "which of my nodes is doing nothing?" is a question a fleet view exists to answer, and the daemon already knows. That figure SHALL NOT be labelled in a way that collides with the `idle` engine state, which means something different. A node whose daemon reports no activity yet SHALL omit the figure rather than imply an engine has sat unused since it started.

#### Scenario: Mixed fleet renders every node

- **WHEN** `spinloop status` runs against a fleet of several nodes
- **THEN** the output has one row per node showing its state, version, and what it serves

#### Scenario: A named environment is read by the same command

- **WHEN** `spinloop status --env prod` runs
- **THEN** that environment's row is rendered with no fleet file required, and
  it reads the same as the row for the same environment named in a fleet file

#### Scenario: An unhealthy endpoint reads as not ready

- **WHEN** `spinloop status --env prod` runs and the control plane reports the
  endpoint unhealthy
- **THEN** the row carries the same not-ready mark a daemon node carries when
  its engine has not answered its health check

#### Scenario: A node reports how long since it last did work

- **WHEN** `spinloop status` runs against a node whose daemon reports a last-active time
- **THEN** that node's row shows how long ago that was, labelled so it is not confused with the `idle` engine state

#### Scenario: A node with no recorded activity omits the figure

- **WHEN** a node's daemon reports no last-active time, because its engine has done no work yet
- **THEN** that node's row shows no activity figure rather than a misleading one

#### Scenario: Version is shown per node

- **WHEN** `spinloop status` runs against a fleet of running nodes
- **THEN** each node's row includes the spinloop version string from its daemon

#### Scenario: Version is omitted for unreachable nodes

- **WHEN** a node's daemon is unreachable
- **THEN** that node's row shows its failure outcome without a version

#### Scenario: The fleet-scoped spelling names its replacement

- **WHEN** the operator runs `spinloop fleet status`
- **THEN** it fails naming `spinloop status` as the command that replaced it

### Requirement: Fleet dashboard

`spinloop dashboard` SHALL open an interactive, full-screen view of the resolved target:
one panel per node, arranged in a grid and refreshed
continuously without operator input. It is the one place where what the fleet is
doing and acting on it meet: unlike `fleet metrics --watch`, it takes keyboard
input and drives the node the operator has selected.

The target SHALL resolve as it does for every other command that acts on a
fleet, so a single named environment opens as a board of one panel. The
dashboard SHALL be openable and usable from cold — a fleet where nothing is up:
every panel SHALL show its node's outcome and reason rather than metrics, and a
node SHALL be startable from within the dashboard.

A problem with the target itself (a missing or unparseable fleet file, an
environment that is not registered) SHALL fail the command before the view
opens, as it does for the other commands that act on a fleet; a problem
with any node SHALL NOT.

#### Scenario: A mixed fleet renders every node

- **WHEN** the operator opens the dashboard against a fleet with some nodes
  answering and some not
- **THEN** every node has a panel: answering nodes show their metrics, the others
  show their outcome and reason, and the view keeps running

#### Scenario: A named environment opens as one panel

- **WHEN** the operator runs `spinloop dashboard --env prod` in a directory
  holding no fleet file
- **THEN** the view opens on that environment alone, and the absent file is not
  an error

#### Scenario: Opening on a fleet where nothing is up

- **WHEN** the operator opens the dashboard and no node is reachable
- **THEN** every panel shows its node's outcome and reason
- **AND** the dashboard does not exit, and starting a node from it works

#### Scenario: A fleet-file problem fails before the view

- **WHEN** the named fleet file is missing or unparseable
- **THEN** the command fails naming the problem, and no interactive view opens

#### Scenario: A non-interactive context is refused

- **WHEN** the dashboard is run with its input or output not on a terminal, such
  as through a pipe or in the background
- **THEN** it fails with a message pointing at `fleet metrics --watch`, and it
  does not enter raw terminal mode or emit screen escapes

#### Scenario: The fleet-scoped spelling names its replacement

- **WHEN** the operator runs `spinloop fleet dashboard`
- **THEN** it fails naming `spinloop dashboard` as the command that replaced it
