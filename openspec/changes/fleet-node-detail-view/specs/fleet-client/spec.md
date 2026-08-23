## ADDED Requirements

### Requirement: Node detail view opens and closes

From the grid, `<enter>` on the selected node SHALL open a full-screen detail
view of that node, replacing the grid; `<esc>` SHALL close the detail view and
return to the grid with the same node still selected. Opening or closing the
detail view SHALL NOT interrupt the fleet's refresh or any action in flight on
any node.

The detail view SHALL be available for any node, including one whose last
answer was a failure (unreachable, unauthorised, a configuration error): the
detail view SHALL show what it can for that node rather than refusing to open.

#### Scenario: Opening the detail view

- **WHEN** the operator selects a node in the grid and presses enter
- **THEN** the grid is replaced by a full-screen view of that node

#### Scenario: Closing the detail view

- **WHEN** the operator presses escape while the detail view is open
- **THEN** the grid is shown again with the same node selected as before

#### Scenario: The detail view opens on a failing node

- **WHEN** the operator opens the detail view for a node that is unreachable or
  otherwise failing
- **THEN** the view opens and shows the node's outcome and reason in place of
  metrics, rather than refusing to open

#### Scenario: Background refresh continues behind the detail view

- **WHEN** the detail view is open
- **THEN** every node in the fleet, including nodes other than the one shown,
  keeps refreshing on its normal cadence, and any action already in flight on
  another node keeps running

### Requirement: The detail view shows the node's metrics and log

The detail view SHALL be laid out as three stacked sections: the node's
metrics, the node's engine log, and a line naming the keys the view answers
to. The metrics section SHALL show the same facts, in the same wording, that
the node's tile and `fleet metrics` show for it, without the tile's clipping —
a fact that does not fit the tile SHALL be shown in full here. The metrics
section SHALL update on the node's normal refresh cadence, exactly as the
tile does.

The log section SHALL show the node's engine log, tailing and following it
the same way `outfit fleet logs -f <node>` follows one node's log, so new
output appears while the view is open without the operator asking for it. A
node whose engine has not run yet, or that cannot supply its log, SHALL show
the same explanation the `fleet logs` command gives for it, rather than an
empty pane.

#### Scenario: Metrics match the tile

- **WHEN** the detail view is open on a node with a running engine
- **THEN** the metrics section shows the same state, serving facts, resource
  usage and counters the node's tile shows, in full rather than clipped

#### Scenario: The log pane follows new output

- **WHEN** the node's engine writes to its log while the detail view is open
- **THEN** the new lines appear in the log section without the operator
  pressing any key

#### Scenario: A node with no log yet

- **WHEN** the detail view is open on a node whose engine has never run
- **THEN** the log section shows the same explanation `fleet logs` gives for
  that node, not an empty pane

### Requirement: The detail view drives the node like the grid

While the detail view is open, the operator SHALL be able to start, stop and
abort the node it shows, through the same confirmation and in-flight rules
the grid applies to the selected node. The node's in-flight status — the verb
and the action's status lines — SHALL be shown in place of its metrics for
the life of the action, the same as the tile shows it. The quit key and
interrupt SHALL exit the dashboard from inside the detail view exactly as
they do from the grid.

#### Scenario: Starting a node from its detail view

- **WHEN** the operator issues the start from the detail view of a stopped node
- **THEN** the start is sent without a prompt and the view shows its progress
  in place of the metrics section until it finishes

#### Scenario: Stopping asks for confirmation from the detail view

- **WHEN** the operator issues the stop from the detail view
- **THEN** the view asks for confirmation before sending it, the same as the
  grid does

#### Scenario: Quitting from the detail view

- **WHEN** the operator presses the quit key, or interrupts, while the detail
  view is open
- **THEN** the dashboard exits the same way it does from the grid, with the
  terminal restored
