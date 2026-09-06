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

While an action is in flight, the node's own tile SHALL carry it: the verb, the
action's current situation, and how long the action has been running, beside the
node's last report rather than in place of it — the action's own account says
what the operator asked for, the report says what the node is doing. For a node
whose last completed refresh answered, the tile SHALL show that answer's state,
what it serves, its last-active record, and its resource usage and token and
request counters whenever the answer carries them, whatever the node's state — a
boot half done is already measuring, and that is the truth the tile keeps showing
while the call works. A node whose action reports nothing and whose refresh has
not yet answered SHALL show the verb alone, and a latest refresh that failed
SHALL change nothing on the tile. Each finished action SHALL clear the action
from its node's tile, which is then the node's report alone, and leave its
outcome on the status line, the one-shot wording.

An action SHALL have exactly one current situation at a time, and a new one
SHALL replace its predecessor outright rather than being added to it. A
situation that was true of an attempt the action has moved on from SHALL NOT be
shown: the tile reports what the action is doing now, never what it was doing
before. In particular, a start refused for want of capacity SHALL stop reporting
that refusal once a further attempt is under way — so a tile never shows a
capacity wait beside a refresh reporting the node up and running.

Anything the tile says about time SHALL be computed when the tile is drawn, not
when the situation it describes arose: a wait counts down towards the attempt it
is waiting for, and an action counts up from when the operator issued it. A
start's situation can hold unchanged for minutes — the attempt that succeeds
keeps one request open for the whole boot and reports nothing while it does — so
these are what distinguish an action that is waiting from one that is wedged.

An action in flight SHALL also carry a spinner beside its verb, whose frame is
likewise chosen when the tile is drawn, and the board SHALL redraw often enough
for it to turn. It says the same thing as the elapsed time to an operator
glancing rather than reading, and it says it on a tile whose every other line
can hold still for minutes.

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

The dashboard's key help SHALL name the start key for the node it describes
only when that node has no action in flight and the board's current read of it
does not report it running, and SHALL name the stop key for that node only when
it has no action in flight and the board's current read of it reports it
running, so the operator is never invited to press a key that would do nothing
there. A read reports its node running when it answered and carries a running
state; a node whose current read reports no state at all — no read has
answered yet, or its newest read failed — SHALL be offered the start key rather
than the stop key, since the board cannot say the node is running and start is
the key that might still do something on it. A node that could not become a
node at all SHALL be offered neither key, since neither would drive anything on
it.

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

- **WHEN** the operator starts a node whose start reports its situation as it
  works
- **THEN** the node's tile shows the verb and the start's current situation
  while the start is in flight
- **AND** a refresh that answers while the start is in flight shows its state
  and whatever it measures on the same tile, beneath the start's own account
- **AND** when the start finishes, the tile is the node's report alone and the
  outcome is on the status line

#### Scenario: A refused start does not outlive its refusal

- **WHEN** a node's start is refused for want of capacity, and a later attempt
  is issued once capacity is free
- **THEN** the tile reports the capacity wait while it holds, and stops
  reporting it once the next attempt is under way, rather than showing it
  beside a refresh reporting the node running

#### Scenario: A start's elapsed time keeps moving

- **WHEN** a start is in flight and its situation does not change for some time
- **THEN** the elapsed time beside the verb keeps advancing on the tile, so the
  operator can tell the start is still waiting rather than wedged

#### Scenario: A wait counts down and an action counts up

- **WHEN** a start is waiting for its next attempt and the operator watches
  without pressing anything
- **THEN** the time until that attempt counts down on the tile, and the time
  since the operator issued the start counts up, both advancing as the tile is
  redrawn rather than standing at the values they held when the wait began
- **AND** the spinner beside the verb turns while they do

#### Scenario: An in-flight start before any report

- **WHEN** the operator starts a node before any refresh of it has answered
- **THEN** its tile shows the verb and the start's own account alone
- **AND** the first answered refresh appears on the tile beside them

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

#### Scenario: The key help hides start and stop where they would do nothing

- **WHEN** the node under the cursor (or shown in the detail view) has no
  action in flight and the board's current read of it reports it running
- **THEN** the key help names the stop key and does not name the start key
- **WHEN** that node has no action in flight and the board's current read of
  it reports it not running
- **THEN** the key help names the start key and does not name the stop key
- **WHEN** that node has an action in flight, or could not become a node at all
- **THEN** the key help names neither the start key nor the stop key

#### Scenario: The key help offers start when the node's state is unknown

- **WHEN** the node under the cursor has no action in flight, and no read of
  it has answered yet, or its newest read failed
- **THEN** the key help names the start key and does not name the stop key

#### Scenario: An action that fails keeps the dashboard open

- **WHEN** an action is sent to a node that cannot be reached, or the node
  refuses it
- **THEN** the failure is shown in the status line with the daemon's own reason
  and the dashboard keeps running with its refreshes
