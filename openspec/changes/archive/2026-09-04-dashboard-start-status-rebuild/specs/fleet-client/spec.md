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
  beside a refresh that reports the node running

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

#### Scenario: An action that fails keeps the dashboard open

- **WHEN** an action is sent to a node that cannot be reached, or the node
  refuses it
- **THEN** the failure is shown in the status line with the daemon's own reason
  and the dashboard keeps running with its refreshes

### Requirement: The fleet refreshes without stalling

The dashboard SHALL refresh the fleet continuously, and SHALL also refresh
immediately on the operator's request. The cadence SHALL be by node kind: a
local daemon machine SHALL refresh on a short interval — seconds, not the
watch mode's minute — and a `kind: remote` environment SHALL refresh on a
much slower cadence, a 60-second interval, one status call a minute, because
its status is a signed call through the cloud control plane rather than a
local socket, and its state changes on the scale of minutes. A manual refresh
SHALL read every node, whether or not its kind's cadence was due. A refresh of a
group SHALL read that group's nodes concurrently, as fan-out does elsewhere.

A node with an action in flight SHALL refresh on the short interval whatever its
kind, and SHALL return to its kind's own cadence once the action settles. The
reasons a cloud environment is read once a minute — the call is expensive and
its state changes slowly — hold for an environment nobody is touching and hold
for neither one the operator has just started.

Every reading of a node SHALL carry the time it was taken, and the dashboard
SHALL NOT show a reading older than the one it is already showing for that node.
Reads are concurrent and of uneven duration, so a reading can land after one
taken later than it; showing it would replace what the node is doing with what
it was doing.

A node whose newest reading has aged well past its cadence SHALL be shown as
such — its age on the panel, and its health unknown rather than whatever the
aged reading said. A stale reading is not a wrong reading, but presenting it
indistinguishably from a current one is: the operator SHALL be able to tell how
old what they are looking at is.

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

#### Scenario: A node being acted on is read more often

- **WHEN** the operator starts a cloud environment
- **THEN** that environment is re-read on the short interval for as long as the
  start is in flight, and returns to its 60-second cadence once the start
  settles
- **AND** the other cloud environments keep their own cadence throughout

#### Scenario: A late reading does not overwrite a newer one

- **WHEN** a read of a node is issued, a later read of the same node answers
  first, and the earlier read then answers
- **THEN** the panel keeps showing the later read, and the earlier one is
  discarded

#### Scenario: A panel with an out-of-date reading shows its age

- **WHEN** a node stops answering and its newest reading ages well past its
  cadence
- **THEN** its panel shows how old that reading is and its health reads
  unknown, rather than continuing to present the aged state as current
- **WHEN** the node answers again
- **THEN** the panel returns to showing its state without an age

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

### Requirement: Dashboard panels show a health indicator

Each panel SHALL show a coloured status glyph alongside its node's name,
distinct from the border colour that marks the selected panel, so a node's
health reads at a glance across a grid of many panels without reading each
panel's text. The glyph SHALL be shown in every panel shape: a settled
answer, an action in flight, a panel awaiting its first refresh, and a panel
showing a failed outcome.

A tile SHALL draw its first line — the glyph, the node's name, and what the
node is doing — as a header bar: light text on one background colour running
the tile's full width, so the name reads as the panel's title rather than as
another line of the panel's body. That background SHALL be a single neutral
colour, the same on every tile, so it never competes with the glyph's own
colour for what a node's health is read from.

A node's health SHALL fall into exactly one of five tiers: healthy,
attention, unhealthy, not serving, and unknown. Healthy is coloured green,
attention yellow, and unhealthy red. Not serving and unknown are both
coloured grey and read as "nothing to watch" rather than "something is
wrong"; the two stay apart by their marks — not serving is the same filled
dot as the other tiers in a faded shade, and unknown keeps its `?` — so a
node known to be undeployed never reads as a node the dashboard has not
heard from yet.

- **Healthy**: the node answered its last refresh, that answer is current, its
  engine is not crashed, not `idle`, not `stopped`, and not `undeployed`, and —
  when the daemon reports readiness for it — the engine is ready. A `running`
  node whose daemon reports no readiness (an older daemon, or a runner with no
  known health check) counts as healthy too, rather than reporting a health tier
  the daemon cannot actually back.
- **Attention**: the node has a start or stop action in flight for it, or is
  `running` with its daemon explicitly reporting the engine not yet ready.
- **Unhealthy**: the node's engine has crashed, or its last refresh's
  outcome was a failure (`unreachable`, `unauthorized`, `config-error`,
  `failed`, or `unsupported`).
- **Not serving**: the node answered its last refresh, that answer is current,
  no action is in flight for it, and its engine is not serving — its state is
  `idle`, the daemon has started nothing, `stopped`, a daemon engine that was
  stopped, or `undeployed`, a remote environment with no instance at all.
- **Unknown**: no current status can be determined for the node — it has not yet
  answered any refresh, its last refresh answered without reporting an engine
  state, or its newest answer has aged well past its cadence and no longer
  describes the node now.

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

#### Scenario: A node whose answer has gone stale reads unknown

- **WHEN** a node last answered `running` and has not answered since, long
  enough that the answer no longer describes the node now
- **THEN** its panel's status glyph is grey rather than the green that answer
  earned when it was current

#### Scenario: An action in flight reads attention

- **WHEN** the operator starts or stops a node and that action has not yet
  finished
- **THEN** that node's panel's status glyph is yellow while the action is in
  flight, whatever the node's last completed refresh reported

#### Scenario: An undeployed node reads not serving

- **WHEN** a node's last completed refresh reports its engine `idle` and no
  start or stop is in flight for it
- **THEN** its panel's status glyph is the faded grey dot, not the green of
  a serving node

#### Scenario: A stopped node reads not serving

- **WHEN** a node's last completed refresh reports its engine `stopped` — a
  daemon engine that was stopped — and no start or stop is in flight for it
- **THEN** its panel's status glyph is the faded grey dot, not the green of
  a serving node

#### Scenario: An undeployed remote environment reads not serving

- **WHEN** a remote environment's last completed refresh reports it
  `undeployed` — it has no instance at all — and no start or stop is in
  flight for it
- **THEN** its panel's status glyph is the faded grey dot, not the green of
  a serving node

#### Scenario: An undeployed node with a start in flight reads attention

- **WHEN** a node's last completed refresh reports its engine `idle` or
  `stopped` and a start for it has not yet finished
- **THEN** its panel's status glyph is yellow while the start is in flight,
  never the grey of not serving

#### Scenario: Not serving and unknown keep different marks

- **WHEN** a grid holds both a node that has answered with `idle` and a node
  that has not yet answered any refresh
- **THEN** the first shows the faded grey filled dot and the second the grey
  `?`, so the two greys are told apart by their mark

The board SHALL carry a title bar of its own along the top of the frame, on
the same surface a tile's header bar uses: the product's name, the screen in
view, and — pushed to the right — the fleet file and what is on that screen. A
terminal too narrow for both halves SHALL keep the left one rather than wrap
the bar onto a second row. The grid and the detail view SHALL draw their title
bar from one place, so the two screens cannot title themselves differently.

The board SHALL use exactly one accent colour, the mint of the spinloop logo,
and SHALL use it only where nothing about a node is being reported: the title
bar's product name, and the border of the selected panel. A node's own state —
the health glyph, the resource bars — SHALL keep the terminal's green, amber
and red, which say what an engine is doing rather than whose product this is.

In the frame's key help, each entry SHALL be drawn as the key and what that
key does: the key in the terminal's own text colour, and what it does a step
back in the muted ink, so the keys are what a glance over the footer picks out.
The prose beside them — a confirmation's question, an action's outcome — SHALL
be left as it is.

#### Scenario: A key stands out from what it does

- **WHEN** the key help is drawn
- **THEN** each key keeps the terminal's own text colour and what it does is
  drawn in the muted ink beside it
- **AND** the status line and a confirmation's question are not drawn that way

#### Scenario: The selected panel is marked in the accent

- **WHEN** the operator moves the selection onto a panel
- **THEN** that panel's border is drawn in the brand accent, and no colour that
  reports a node's state is used for it

#### Scenario: The title bar names the product and the screen

- **WHEN** the dashboard is open on the grid or on a node's detail view
- **THEN** the top of the frame carries a title bar naming the product and that
  screen, with the fleet file and the screen's own details to the right

#### Scenario: A narrow terminal keeps the left half of the title bar

- **WHEN** the terminal is too narrow for both halves of the title bar
- **THEN** the product and the screen are kept and the right half is dropped,
  and the bar stays one row

#### Scenario: The header bar does not carry the health colour

- **WHEN** two nodes of different health are shown side by side
- **THEN** both tiles' header bars are the same colour, and the two nodes are
  told apart by their glyphs

#### Scenario: The glyph is distinct from the selection border

- **WHEN** the operator moves the selection onto a panel
- **THEN** the selected panel's border colour changes as it does today, and
  every panel's status glyph colour is unaffected by which panel is selected
