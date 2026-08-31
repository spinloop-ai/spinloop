# fleet-client Specification

## Purpose
Define the `spinloop fleet` command family: the client that reads `fleet.yaml`,
polls each node's daemon control API, and renders the cluster — observing every
engine and driving individual ones, degrading gracefully when a node cannot be
reached.
## Requirements
### Requirement: Fleet status

`spinloop fleet status` SHALL query every node's daemon status endpoint and render one row per node: the node name, its engine state (`idle`/`running`/`stopped`/`crashed`), what it is serving (runner and model when known), the spinloop version of the daemon on that node, and its reachability. Nodes SHALL be queried concurrently so the command's latency is that of the slowest reachable node, not their sum.

A node SHALL also report how long it has been since its engine last did work, taken from the activity its daemon tracks — "which of my nodes is doing nothing?" is a question a fleet view exists to answer, and the daemon already knows. That figure SHALL NOT be labelled in a way that collides with the `idle` engine state, which means something different. A node whose daemon reports no activity yet SHALL omit the figure rather than imply an engine has sat unused since it started.

#### Scenario: Mixed fleet renders every node

- **WHEN** `spinloop fleet status` runs against a fleet of several nodes
- **THEN** the output has one row per node showing its state, version, and what it serves

#### Scenario: A node reports how long since it last did work

- **WHEN** `spinloop fleet status` runs against a node whose daemon reports a last-active time
- **THEN** that node's row shows how long ago that was, labelled so it is not confused with the `idle` engine state

#### Scenario: A node with no recorded activity omits the figure

- **WHEN** a node's daemon reports no last-active time, because its engine has done no work yet
- **THEN** that node's row shows no activity figure rather than a misleading one

#### Scenario: Version is shown per node

- **WHEN** `spinloop fleet status` runs against a fleet of running nodes
- **THEN** each node's row includes the spinloop version string from its daemon

#### Scenario: Version is omitted for unreachable nodes

- **WHEN** a node's daemon is unreachable
- **THEN** that node's row shows its failure outcome without a version

### Requirement: Unreachable nodes degrade

A node that does not yield a result SHALL be shown with a typed outcome and a
short reason, distinguishing what went wrong:

- `unreachable` — the daemon could not be contacted at all (connection
  refused, timeout, DNS failure);
- `unauthorized` — the daemon rejected the client's bearer token;
- `config-error` — the node could not be called, typically a token reference
  that resolves to nothing;
- `failed` — the daemon answered with an error (a refused start, an
  unservable config): the node is healthy, the request was not.

Such a node SHALL NOT abort the command: every other node's result SHALL still
render, and the command SHALL succeed.

#### Scenario: One node down, the rest still shown

- **WHEN** `spinloop fleet status` runs and one node's daemon is unreachable
- **THEN** that node's row reads `unreachable` with its reason, the other nodes
  render normally, and the command exits successfully

#### Scenario: Bad token is distinguished from unreachable

- **WHEN** a node's daemon rejects the client's bearer token
- **THEN** that node's row reads `unauthorized`, not `unreachable`

#### Scenario: A refused request is distinguished from an unreachable node

- **WHEN** a node's daemon answers a request with an error, such as a start
  while its engine is already running
- **THEN** that node's outcome reads `failed` with the daemon's own message,
  not `unreachable` — the node was reached, the request was refused

### Requirement: Fleet metrics

`spinloop fleet metrics` SHALL query every node's metrics endpoint and render
each node's engine and system metrics using the same bar, table, and json
formats `spinloop remote metrics` provides, selected by `--format`. Unreachable
nodes SHALL be reported as in status rather than omitted. The command SHALL
support a `--watch`/`-w` mode that refreshes on an interval, clearing and
redrawing the screen in place with no scrollback accumulation, and exiting
cleanly on interrupt.

#### Scenario: Bar format per node

- **WHEN** `spinloop fleet metrics` runs without `--format`
- **THEN** each reachable node's metrics render in bar format under its name

#### Scenario: JSON aggregates the fleet

- **WHEN** `spinloop fleet metrics --format=json` runs
- **THEN** the output is valid JSON keyed or labelled by node, including
  unreachable nodes with their error

#### Scenario: Watch redraws in place

- **WHEN** `spinloop fleet metrics --watch` runs
- **THEN** each refresh clears the screen and redraws the fleet, and Ctrl+C
  exits cleanly

### Requirement: Driving one node

`spinloop fleet start <node>` and `spinloop fleet stop <node>` SHALL call the named
node's daemon start and stop endpoints. Start and stop SHALL require a node
name: invoked without one they SHALL fail and list the available nodes, rather
than acting on the whole fleet. An unknown node name SHALL fail, naming the
known nodes. The daemon's own rules still hold — a start while that node's
engine is running is reported as the daemon's conflict, and a stop is
idempotent.

#### Scenario: Start a named node

- **WHEN** `spinloop fleet start gpu-box` runs and that node is idle
- **THEN** the client calls that node's daemon start endpoint and reports the
  resulting state

#### Scenario: Start with no node names the fleet

- **WHEN** `spinloop fleet start` runs with no node argument
- **THEN** it fails, listing the nodes, and starts nothing

#### Scenario: Unknown node

- **WHEN** `spinloop fleet stop nope` runs and no node is named `nope`
- **THEN** it fails, naming the known nodes, and stops nothing

### Requirement: Authenticated fan-out

Every request the client makes to a node SHALL carry that node's resolved
bearer token when one is configured, as the daemon control API requires. A
node configured with a token whose env var is unset SHALL be reported as a
configuration error for that node (distinct from an unreachable node), without
aborting the rest of the fleet.

#### Scenario: Missing token env var is a per-node error

- **WHEN** a node references a token env var that is not set
- **THEN** that node reports a configuration error and the other nodes still
  render

### Requirement: Fleet logs

`spinloop fleet logs` SHALL read the engine output of the fleet's nodes through
each node's daemon, so "what did that engine say?" is answerable from the same
place as "what is it doing?" — without shell access to any machine. With no node
named it SHALL read every node in the fleet; naming a node SHALL restrict it to
that one. Nodes SHALL be read concurrently, so the command's latency is that of
the slowest reachable node rather than their sum.

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

### Requirement: Fleet log lines are attributed to their node

When output from more than one node is printed, every line SHALL identify the
node it came from, since interleaved output from several machines is misleading
otherwise. When the output is from a single node — because the fleet holds one,
or because a node was named — that attribution SHALL be omitted, so the common
case of reading one node reads like that node's own log.

Lines SHALL NOT be interleaved across nodes by time: the daemon returns captured
output as the engine wrote it, and an engine's output carries no timestamp the
client can rely on, so each node's output SHALL be kept in its own order rather
than merged into a false chronology.

#### Scenario: Several nodes are labelled

- **WHEN** output from more than one node is printed
- **THEN** each line identifies its node

#### Scenario: A single node is not labelled

- **WHEN** every printed line comes from one node
- **THEN** the lines carry no node prefix

### Requirement: Fleet logs can be followed

`spinloop fleet logs` SHALL be able to keep running and print output as nodes
produce it, rather than exiting after one read. Following SHALL resume each node
from the position that node last returned, so a line already printed is never
printed twice. Interrupting SHALL exit cleanly, without reporting an error.

#### Scenario: New output appears

- **WHEN** the operator follows the fleet's logs and a node's engine writes more
  output
- **THEN** that output is printed as it arrives, attributed to its node

#### Scenario: No duplicates across polls

- **WHEN** following continues across several polls
- **THEN** no line that has already been printed is printed again

#### Scenario: Interrupting stops cleanly

- **WHEN** the operator interrupts a follow
- **THEN** the command exits without reporting an error

### Requirement: A node that cannot supply logs does not fail the command

Reading logs SHALL degrade per node in the same way the rest of the fleet
commands do: a node that cannot be reached, that rejects the client's
credentials, that has never run an engine, or whose daemon is too old to serve
logs SHALL be reported against that node while every other node's output is
still printed. The command SHALL NOT fail as a whole because one node could not
answer.

#### Scenario: One node is unreachable

- **WHEN** one node cannot be reached and the others can
- **THEN** the reachable nodes' output is printed
- **AND** the unreachable node is reported as such

#### Scenario: A node has never run an engine

- **WHEN** a node has no engine log because nothing has ever run there
- **THEN** that is reported for that node, distinctly from a node that failed
  to answer

#### Scenario: A node's daemon predates the endpoint

- **WHEN** a node's daemon does not serve the logs endpoint
- **THEN** that node is reported as needing an upgrade, naming what is missing
- **AND** the other nodes' output is unaffected

### Requirement: Explaining a route

`spinloop fleet route` SHALL report the node a harness launch would choose for a
given Spinloop, the endpoint that node resolves to, and why it was chosen — and
SHALL change nothing: it SHALL never push a config, start an engine, or write a
harness config. It is how a routing decision is checked before an agent depends
on it, and how an unexpected choice is diagnosed after one.

When no node would be chosen, it SHALL report each node's state and the reason
it was passed over, and SHALL name what would happen on a real launch: which
node would be woken, or that none could serve it.

The Spinloop and the fleet file SHALL resolve as they do for a launch: the Spinloop
path defaults to `./Spinloop`, and `--fleet` overrides the Spinloop's `FLEET`. It
SHALL accept `--prefer` and `--node` as a launch does, and SHALL name the
activity preference in force — comparing the two preferences on a live fleet is
the cheapest way to decide which one a fleet should be run with.

#### Scenario: The chosen node is explained

- **WHEN** `spinloop fleet route` runs against a fleet with a node serving the
  Spinloop's model
- **THEN** it prints that node, its resolved engine endpoint, and why it was
  chosen

#### Scenario: Routing changes nothing

- **WHEN** `spinloop fleet route` runs against a fleet where no node is serving
  the Spinloop's model
- **THEN** no engine is started, no config is pushed, and no harness config is
  written

#### Scenario: The two preferences can be compared

- **WHEN** `spinloop fleet route --prefer active` runs against a fleet whose file
  declares `prefer: idle`
- **THEN** it reports the node `active` would choose and names that preference,
  without changing the fleet file

#### Scenario: A launch that would wake a node says so

- **WHEN** `spinloop fleet route` runs and no node is serving the model but one
  could
- **THEN** it names the node a launch would wake, and does not wake it

### Requirement: Fleet dashboard

`spinloop fleet dashboard` SHALL open an interactive, full-screen view of the fleet:
one panel per node in the fleet file, arranged in a grid and refreshed
continuously without operator input. It is the one place where what the fleet is
doing and acting on it meet: unlike `fleet metrics --watch`, it takes keyboard
input and drives the node the operator has selected.

The fleet file SHALL resolve as it does for the rest of the fleet commands. The
dashboard SHALL be openable and usable from cold — a fleet where nothing is up:
every panel SHALL show its node's outcome and reason rather than metrics, and a
node SHALL be startable from within the dashboard.

A problem with the fleet file itself (missing, unparseable) SHALL fail the
command before the view opens, as it does for the other fleet commands; a problem
with any node SHALL NOT.

#### Scenario: A mixed fleet renders every node

- **WHEN** the operator opens the dashboard against a fleet with some nodes
  answering and some not
- **THEN** every node has a panel: answering nodes show their metrics, the others
  show their outcome and reason, and the view keeps running

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

### Requirement: Dashboard panels show the node's metrics

Each panel SHALL show, for a node that answered the last completed refresh, the
same facts the bar format of `fleet metrics` renders for that node: its state,
what it serves (runner and model when known), how long since it last did work
(with the same labelling rules as the rest of the fleet surfaces), its resource
usage, and its token and request counters. A panel SHALL show the answer of the
last completed refresh for that node — not a mix of refreshes and not a stale
bar with a fresh outcome.

A panel SHALL degrade gracefully when a node answers with fewer facts (no system
stats, no GPUs, an engine that is not running) rather than failing to render.
A node whose answer is a failure — unreachable, unauthorised, a configuration
error — SHALL show its typed outcome and reason in its panel instead of metrics.

One node SHALL be selected at a time, and the selected panel SHALL be
distinguishable at a glance from the others. Selection SHALL move according to
the grid the dashboard is currently rendering, not the fleet file's flat
order: the up and down keys SHALL move the selection to the tile directly
above or below it, in the same column of the adjacent row; the left and right
keys SHALL move the selection to the adjacent tile in the same row. None of
the four directions SHALL wrap — a move off the grid's edge SHALL leave the
selection where it was. A resize that changes how many tiles fit per row
SHALL be reflected immediately: the next move follows the new grid, not the
one the dashboard was drawing before the resize.

#### Scenario: A running node's panel updates

- **WHEN** a node's engine is running and its counters or utilisation change
- **THEN** the node's panel shows the new figures on a subsequent refresh,
  without the operator pressing any key

#### Scenario: A node that reports fewer facts still renders

- **WHEN** a node's answer carries no GPU or system statistics
- **THEN** its panel shows the facts it has, with the missing ones absent rather
  than an error

#### Scenario: A failing node's panel shows why

- **WHEN** a node cannot be reached, or its daemon rejects the client
- **THEN** its panel shows the node's outcome and reason, and the other panels
  are unaffected

#### Scenario: Selection moves and is visible

- **WHEN** the operator moves the selection with the navigation keys
- **THEN** the selection moves according to the currently rendered grid and
  the selected panel is visibly distinct from the others

#### Scenario: Up and down move by grid row

- **WHEN** the dashboard renders more than one tile per row and the operator
  presses the down key
- **THEN** the selection moves to the tile directly below it, in the same
  column of the next row, rather than to the next tile in the fleet file's
  order

#### Scenario: Left and right move within a row

- **WHEN** the operator presses the right key
- **THEN** the selection moves to the next tile in the same row
- **AND** pressing the left key from there returns the selection to the tile
  it started on

#### Scenario: Movement clamps at the grid's edges

- **WHEN** the selection is on the top row and the operator presses up, on the
  bottom row and presses down, on the first column and presses left, or on the
  last column and presses right
- **THEN** the selection does not move and does not wrap to the opposite edge

#### Scenario: Down clamps on a short last row

- **WHEN** the selection is in a column that the last, partially-filled grid
  row does not reach, and the operator presses down
- **THEN** the selection moves to the last tile that exists in that row
  instead of moving past the end of the fleet

#### Scenario: A resize changes the grid the arrow keys follow

- **WHEN** the terminal is resized so the dashboard now fits a different
  number of tiles per row, and the operator then presses an arrow key
- **THEN** the selection moves according to the new column count, not the one
  in effect before the resize

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
and the action's status lines as the call reports them, beside the node's last
report rather than in place of it — the call's lines say what the operator
asked for, the report says what the node is doing. For a node whose last
completed refresh answered, the tile SHALL show that answer's state, what it
serves, its last-active record, and its resource usage and token and request
counters whenever the answer carries them, whatever the node's state — a boot
half done is already measuring, and that is the truth the tile keeps showing
while the call works. A node whose call reports nothing and whose refresh has
not yet answered SHALL show the verb alone, and a latest refresh that failed
SHALL change nothing on the tile. Each finished action SHALL clear the action
from its node's tile, which is then the node's report alone, and leave its
outcome on the status line, the one-shot wording.

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
- **AND** a refresh that answers while the start is in flight shows its state
  and whatever it measures on the same tile, beneath the start's lines
- **AND** when the start finishes, the tile is the node's report alone and the
  outcome is on the status line

#### Scenario: An in-flight start before any report

- **WHEN** the operator starts a node before any refresh of it has answered
- **THEN** its tile shows the verb and the start's status lines alone
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

#### Scenario: An action that fails keeps the dashboard open

- **WHEN** an action is sent to a node that cannot be reached, or the node
  refuses it
- **THEN** the failure is shown in the status line with the daemon's own reason
  and the dashboard keeps running with its refreshes

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
the same way `spinloop fleet logs -f <node>` follows one node's log, so new
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

### Requirement: The fleet refreshes without stalling

The dashboard SHALL refresh the fleet continuously, and SHALL also refresh
immediately on the operator's request. The cadence SHALL be by node kind: a
local daemon machine SHALL refresh on a short interval — seconds, not the
watch mode's minute — and a `kind: remote` environment SHALL refresh on a
much slower cadence, a 60-second interval, one status call a minute, because
its status is a signed call through the cloud control plane rather than a
local socket, and its state changes on the scale of minutes. A manual refresh
SHALL read every node, whether or not its kind's cadence was due. A refresh of a group SHALL read that group's nodes
concurrently, as fan-out does elsewhere.

One node that is slow or unanswerable SHALL NOT hold the rest of the fleet
hostage to it: a refresh SHALL give up on a node that has not answered within
the budget of the node's group, show that node with its outcome for this
round, and the other nodes SHALL keep their cadence. The dashboard SHALL NOT
start a second read of a group while that group's previous one has not
finished; the two groups' reads MAY run at the same time, and a slow cloud
round SHALL NOT stretch the local machines' cadence.

#### Scenario: Continuous refresh without input

- **WHEN** the dashboard is open and no key is pressed
- **THEN** the local panels keep updating on the short interval, and each
  cloud panel is re-read once a minute, on the 60-second cadence

#### Scenario: An immediate refresh

- **WHEN** the operator presses the refresh key
- **THEN** every node is re-read, local and cloud alike, rather than waiting
  for each kind's next cadence

#### Scenario: One hung node does not stall the board

- **WHEN** one node does not answer within its group's budget while the others
  do
- **THEN** that node's panel shows its outcome, the other panels refreshed on
  their normal cadence, and no second read of its group started over the
  in-flight one

#### Scenario: A slow cloud round keeps the local cadence

- **WHEN** a cloud environment is slow to answer while its group's read runs
- **THEN** the local machines keep refreshing on the short interval, and the
  cloud panel shows its outcome on the cloud cadence

### Requirement: The dashboard exits cleanly

The dashboard SHALL quit on its quit key and on interrupt. On every exit path it
SHALL restore the terminal — leaving the alternate screen, showing the cursor
again, and restoring terminal modes — so the operator's shell is immediately
usable, and it SHALL leave no trace in the terminal's scrollback.

#### Scenario: Quitting returns to the shell

- **WHEN** the operator presses the quit key, or interrupts
- **THEN** the dashboard exits without an error and the terminal is in the state
  it was before the dashboard opened, with the shell prompt ready

### Requirement: Dashboard panels show a health indicator

Each panel SHALL show a coloured status glyph alongside its node's name,
distinct from the border colour that marks the selected panel, so a node's
health reads at a glance across a grid of many panels without reading each
panel's text. The glyph SHALL be shown in every panel shape: a settled
answer, an action in flight, a panel awaiting its first refresh, and a panel
showing a failed outcome.

A node's health SHALL fall into exactly one of four tiers, coloured green,
yellow, red, and grey respectively:

- **Healthy**: the node answered its last refresh, its engine is not
  crashed, and — when the daemon reports readiness for it — the engine is
  ready. A `running` node whose daemon reports no readiness (an older
  daemon, or a runner with no known health check) counts as healthy too,
  rather than reporting a health tier the daemon cannot actually back.
- **Attention**: the node has a start or stop action in flight for it, or is
  `running` with its daemon explicitly reporting the engine not yet ready.
- **Unhealthy**: the node's engine has crashed, or its last refresh's
  outcome was a failure (`unreachable`, `unauthorized`, `config-error`,
  `failed`, or `unsupported`).
- **Unknown**: no status can be determined for the node — it has not yet
  answered any refresh, or its last refresh answered without reporting an
  engine state.

#### Scenario: A running, ready node reads healthy

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports the engine ready
- **THEN** its panel's status glyph is green

#### Scenario: A running node still loading reads attention

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports the engine not yet ready
- **THEN** its panel's status glyph is yellow, even though its engine state
  reads `running`

#### Scenario: A running node with no readiness signal reads healthy

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports no readiness for it
- **THEN** its panel's status glyph is green, the same as before this
  daemon-side signal existed

#### Scenario: A crashed node reads unhealthy

- **WHEN** a node's last completed refresh reports its engine `crashed`
- **THEN** its panel's status glyph is red

#### Scenario: An unreachable node reads unhealthy

- **WHEN** a node's last refresh could not reach its daemon
- **THEN** its panel's status glyph is red, alongside the outcome and reason
  already shown

#### Scenario: A node awaiting its first refresh reads unknown

- **WHEN** the dashboard opens and a node has not yet answered any refresh
- **THEN** its panel's status glyph is grey, until its first refresh lands

#### Scenario: An answer without a state reads unknown

- **WHEN** a node's last refresh answered but reported no engine state
- **THEN** its panel's status glyph is grey

#### Scenario: An action in flight reads attention

- **WHEN** the operator starts or stops a node and that action has not yet
  finished
- **THEN** that node's panel's status glyph is yellow while the action is in
  flight, whatever the node's last completed refresh reported

#### Scenario: The glyph is distinct from the selection border

- **WHEN** the operator moves the selection onto a panel
- **THEN** the selected panel's border colour changes as it does today, and
  every panel's status glyph colour is unaffected by which panel is selected
