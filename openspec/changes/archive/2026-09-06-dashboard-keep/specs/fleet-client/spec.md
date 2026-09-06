## ADDED Requirements

### Requirement: The dashboard keeps the selected node

The dashboard SHALL let the operator set the retention deadline of the node
currently selected, from the keyboard, through the same node operation the
one-shot keep command uses. Keep applies to a node's instance — the time until
which the cloud's idle sweep leaves it alone — so it SHALL be offered only for
nodes that support retention, namely the fleet's remote environments; a node
without retention support SHALL take no keep.

The keep key SHALL open a duration prompt rather than send anything. The
prompt SHALL name the node it will apply to, show the duration as the operator
has it, and name its confirm and cancel keys. It SHALL open pre-filled with a
default duration, so that the common case is the key and a confirm, and the
confirm SHALL send the duration as shown, whatever the operator has changed it
to. The duration SHALL be expressed in the same syntax the one-shot keep
command accepts.

A confirm of an entry that is not a positive duration SHALL send nothing: the
prompt SHALL stay open and show why the entry was rejected, so the operator can
correct it and confirm again. A cancel SHALL close the prompt and send nothing.
Once the prompt is open, the navigation and selection keys SHALL drive
nothing: the prompt takes the keyboard until it is confirmed or cancelled.

Keep SHALL not ask for a second confirmation after the prompt: the operator
has typed, or deliberately kept, the duration and chosen to send it, and a
keep overwrites a deadline rather than ending anything. A keep in flight SHALL
be shown on the node's panel like any action — its verb and how long it has
been running — and SHALL NOT be abortable: it is one fast call with no
open-ended wait, and abandoning the wait on it would leave the operator unsure
whether the deadline was set.

A node that already has an action in flight SHALL take no keep; the dashboard
SHALL say it is still working and drive nothing. The key help SHALL name the
keep key only for the node it describes when that node supports retention and
has no action in flight, so the operator is never invited to press a key that
would do nothing there.

The outcome of a keep SHALL be shown on the status line: on success the
deadline the control plane set, on failure the control plane's own reason — a
deployment that predates keep support, an environment with no instance to
retain — and a failed keep SHALL NOT close the dashboard. What the keep
changes — the deadline the node now carries — SHALL appear on the node's panel
through the normal refresh, without the operator asking for it.

The node's detail view SHALL offer keep on the node it shows, through the same
prompt and the same rules the grid applies to the selected node.

#### Scenario: Keep opens a prompt, it does not send

- **WHEN** the operator selects a remote environment with no action in flight
  and issues keep
- **THEN** a duration prompt opens naming the node, pre-filled with a default
  duration, and nothing has been sent

#### Scenario: A confirmed keep sets the deadline

- **WHEN** the operator changes the prompt's duration and confirms it
- **THEN** the node is kept until now plus the confirmed duration and the
  status line shows the deadline the control plane set

#### Scenario: A pre-filled keep is one key and a confirm

- **WHEN** the operator issues keep on a remote environment and confirms the
  prompt without changing its duration
- **THEN** the node is kept until now plus the default duration

#### Scenario: A bad duration sends nothing

- **WHEN** the operator confirms an entry that is not a positive duration
- **THEN** nothing is sent, the prompt stays open, and it shows why the entry
  was rejected

#### Scenario: Cancelling sends nothing

- **WHEN** the operator opens the keep prompt and cancels it
- **THEN** the prompt closes and the node's deadline is unchanged

#### Scenario: The prompt takes the keyboard

- **WHEN** the keep prompt is open and the operator presses a navigation key
- **THEN** the selection does not move and the prompt stays open

#### Scenario: A local node takes no keep

- **WHEN** the operator issues keep on a local daemon node
- **THEN** the dashboard drives nothing: a local daemon has no idle sweep to
  hold off, so there is no deadline to set

#### Scenario: The key help hides keep where it would do nothing

- **WHEN** the node under the cursor is a local daemon node, or has an action
  in flight
- **THEN** the key help does not name the keep key
- **WHEN** that node is a remote environment with no action in flight
- **THEN** the key help names the keep key

#### Scenario: A busy node is not kept again

- **WHEN** the operator issues keep on a node whose start, stop, or keep is
  still in flight
- **THEN** the dashboard drives nothing and says the node is still working

#### Scenario: A keep in flight cannot be aborted

- **WHEN** the operator issues the abort on a node whose keep is in flight
- **THEN** the abort drives nothing: the keep keeps running, the panel keeps
  showing it, and its outcome lands on the status line when the call returns

#### Scenario: A keep that fails keeps the dashboard open

- **WHEN** a keep is sent to an environment whose deployment predates keep
  support, or to one with no instance to retain
- **THEN** the failure is shown on the status line with the control plane's
  own reason and the dashboard keeps running with its refreshes

#### Scenario: Keep from the detail view

- **WHEN** the operator issues keep from the detail view of a remote
  environment
- **THEN** the same prompt opens with the same rules, and its outcome is
  shown as the grid would show it

### Requirement: The dashboard panels show the retention deadline

A node's panel SHALL show the node's retention deadline — the time until which
the idle sweep leaves the node alone — whenever the node's last answered
refresh carries one, worded the same way the one-shot surfaces report it, and
SHALL omit the line when the answer carries none: a node with no active
retention, a node without retention at all, or a read from a control plane
that predates the field. The deadline SHALL appear on the tile and on the
node's detail screen, which draw the same lines, so the two cannot show the
same read differently, and it SHALL sit among the panel's other time facts
rather than displacing the node's state or what it serves.

A deadline that has passed carries no active retention: a refresh taken after
the deadline SHALL not show the line, and a panel SHALL NOT draw a deadline
the sweep has already moved past as though the node were still held.

#### Scenario: A retained node shows its deadline

- **WHEN** a remote environment's refresh answer carries a retention deadline
  in the future
- **THEN** its tile shows the deadline, and its detail screen shows the same
  line

#### Scenario: A node with no retention shows no line

- **WHEN** a node's refresh answer carries no retention deadline
- **THEN** its panel shows no deadline line and renders the rest exactly as
  before

#### Scenario: An older control plane degrades to no line

- **WHEN** a remote environment's control plane predates the deadline in its
  stats reply
- **THEN** its panel shows no deadline line and no error, and the rest of the
  panel is unaffected

#### Scenario: A passed deadline disappears

- **WHEN** a node's retention deadline has passed and a later refresh answers
- **THEN** the panel no longer shows the deadline

#### Scenario: Tile and detail agree

- **WHEN** the operator opens the detail screen of a retained node
- **THEN** the detail screen's deadline line is the same line, worded the
  same, the tile draws for the same read
