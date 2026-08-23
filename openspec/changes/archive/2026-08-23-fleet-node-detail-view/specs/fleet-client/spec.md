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

The operator SHALL be able to pause and resume the log's follow from the
keyboard, independently of the rest of the view: while paused, the log
section SHALL stop picking up new output, and the view SHALL show whether the
log is following or paused. Nothing written while paused is lost: resuming
SHALL fetch and show whatever the engine wrote in the meantime, the same
backlog a poll would have picked up had it never paused. Pausing SHALL NOT
affect the metrics section's own refresh, or any node's start, stop or abort.

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

#### Scenario: Pausing the log

- **WHEN** the operator pauses the log's follow
- **THEN** the log section stops picking up new output and the view shows
  that the log is paused
- **AND** the metrics section keeps refreshing on its own cadence

#### Scenario: Resuming the log

- **WHEN** the operator resumes a paused log
- **THEN** whatever the engine wrote while paused appears in the log section,
  and new output continues to appear as it is written

### Requirement: The detail view drives the node like the grid

While the detail view is open, the operator SHALL be able to start, stop and
abort the node it shows, through the same confirmation and in-flight rules
the grid applies to the selected node. The node's in-flight status — the verb
and the action's status lines — SHALL be shown in place of its metrics for
the life of the action, the same as the tile shows it. The quit key and
interrupt SHALL NOT exit the dashboard from inside the detail view: quitting
is a grid-level action, and the detail view SHALL take the operator back to
the grid on escape rather than closing the dashboard out from under it.

#### Scenario: Starting a node from its detail view

- **WHEN** the operator issues the start from the detail view of a stopped node
- **THEN** the start is sent without a prompt and the view shows its progress
  in place of the metrics section until it finishes

#### Scenario: Stopping asks for confirmation from the detail view

- **WHEN** the operator issues the stop from the detail view
- **THEN** the view asks for confirmation before sending it, the same as the
  grid does

#### Scenario: The quit key does nothing from the detail view

- **WHEN** the operator presses the quit key, or interrupts, while the detail
  view is open
- **THEN** the dashboard keeps running and the detail view stays open
- **WHEN** the operator then presses escape and the quit key
- **THEN** the dashboard exits from the grid, with the terminal restored

## MODIFIED Requirements

### Requirement: The dashboard drives the selected node

The dashboard SHALL let the operator start and stop the engine of the node
currently selected, from the keyboard, through the same node operations the one-
shot `fleet start` and `fleet stop` commands use. An action SHALL reach the
node under the cursor and no other. An action is one per node, not one per
board: while one node is starting, the operator selects another and starts it,
and the wakes run side by side. A node that already has an action in flight
SHALL take no further start or stop; the dashboard says it is still working
and drives nothing.

Start SHALL proceed without confirmation. Stop SHALL require an explicit
confirmation before it is sent, because it ends an engine that may be serving
work: a declined or abandoned confirmation SHALL send nothing.

While an action is in flight, the node's own tile SHALL carry it: the verb
and the action's status lines as the call reports them, in place of the node's
last report — a boot's progress is the node's truth until the report returns.
A node whose call reports nothing SHALL show the verb alone. Each finished
action SHALL clear its node's tile back to the node's next report and leave
its outcome on the status line, the one-shot wording.

The outcome of an action SHALL be shown inside the dashboard (a status line the
operator can read before the next refresh replaces attention), and a refused or
unreachable action SHALL NOT close the dashboard. What an action changes in state
— an engine coming up after a start, coming down after a stop — SHALL appear
through the normal refresh, without the operator asking for it.

A start in flight SHALL be abortable from the keyboard, on the node under the
cursor. A start carries no deadline — a cold cloud wake takes minutes, and a
deadline would report a failure to a slow success — so the operator's abort is
its exit: an abort SHALL end the dashboard's wait on the start, return the
node's tile to the node's report, free the node to be started or stopped again,
and show its outcome on the status line, without closing the dashboard. An abort
ends the wait, not the work it set in motion: the dashboard SHALL NOT present it
as a cancellation of the wake, and what the wake goes on to do — the node's
state — SHALL appear through the normal refresh.

A stop in flight SHALL NOT be abortable: it targets an engine that is already
running rather than a cold wake with no deadline of its own, and abandoning
the dashboard's wait on it would leave the operator unsure whether the stop
still went ahead. The abort key SHALL drive nothing on a node whose in-flight
action is a stop.

The dashboard's key help SHALL name the abort key only while a start is in
flight on the node it describes — the node under the cursor on the grid, the
node in view in the detail view — and SHALL NOT name it for an idle or
running node, or one whose in-flight action is a stop, so the operator is
never invited to press a key that would do nothing there.

#### Scenario: Starting a cold node

- **WHEN** the operator selects a node with no engine running and issues the
  start
- **THEN** the start is sent without a prompt, its outcome is shown in the
  status line, and the panel shows the node's new state as the refreshes come
  around

#### Scenario: Stopping asks first

- **WHEN** the operator issues the stop on the selected node
- **THEN** the dashboard asks for confirmation and nothing is sent
- **WHEN** the operator declines
- **THEN** the stop is not sent and the node's state is unchanged

#### Scenario: A confirmed stop is sent

- **WHEN** the operator issues the stop and confirms
- **THEN** the stop is sent to the selected node only, its outcome is shown in
  the status line, and the panel follows the node's state on subsequent refreshes

#### Scenario: A start is watched on its own tile

- **WHEN** the operator starts a node whose start reports progress as it works
- **THEN** the node's tile shows the verb and the start's status lines while
  the start is in flight
- **AND** when the start finishes, the tile returns to the node's report and
  the outcome is on the status line

#### Scenario: Two nodes wake at once

- **WHEN** the operator starts one node, selects another, and starts it
- **THEN** both wakes run at the same time, each reported on its own tile
- **AND** each finishing action clears its own node and leaves its outcome on
  the status line without disturbing the other

#### Scenario: A node still starting is not started again

- **WHEN** the operator presses start on a node whose start is in flight
- **THEN** the dashboard drives nothing and says the node is still starting

#### Scenario: An in-flight start can be abandoned

- **WHEN** the operator's start on a node is still in flight — the cloud waiting
  for capacity, the boot slow, or the connection behind it dropping and
  retrying — and the operator issues the abort on that node
- **THEN** the dashboard stops waiting on the start, the node's tile returns to
  the node's report, and the outcome is shown on the status line
- **AND** the node may be started or stopped again, and the dashboard keeps
  running with its refreshes

#### Scenario: An abort is not a cancellation of the wake

- **WHEN** the operator aborts a start whose wake the cloud is already carrying
  on
- **THEN** the dashboard reports that it stopped waiting, not that the wake was
  cancelled
- **AND** the node's state, whatever the wake goes on to do, appears through the
  normal refresh

#### Scenario: A stop in flight cannot be aborted

- **WHEN** the operator issues the abort on a node whose stop is in flight
- **THEN** the dashboard drives nothing: the stop keeps running, the tile keeps
  showing it as stopping, and the node's state comes back through the normal
  refresh

#### Scenario: The key help hides abort when nothing is abortable

- **WHEN** the node under the cursor (or shown in the detail view) is idle,
  running with nothing in flight, or has a stop in flight
- **THEN** the key help does not name the abort key
- **WHEN** that node has a start in flight
- **THEN** the key help names the abort key

#### Scenario: An action that fails keeps the dashboard open

- **WHEN** an action is sent to a node that cannot be reached, or the node
  refuses it
- **THEN** the failure is shown in the status line with the daemon's own reason
  and the dashboard keeps running with its refreshes
