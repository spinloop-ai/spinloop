## MODIFIED Requirements

### Requirement: Serve basics

`spinloop serve [path]` SHALL read a Spinloop (default `./Spinloop`, aliases and
directories accepted like every Spinloop command), build the command for the
engine its `PROVIDER` names, print it in copy-pasteable shell form, and run it.
`--dry-run`/`-n` SHALL print the command without launching. A missing binary
SHALL produce an install hint naming **that** engine rather than a raw exec
error.

A run whose stdout is not a terminal SHALL forward the engine's stdout and
stderr to serve's own, exactly as before the view existed. A run on a terminal
SHALL run the engine under the serve view, whose log section is fed by the
engine's captured output (see The serve view); the command serve prints before
running SHALL be written to stderr there, since stdout is the view's screen.

#### Scenario: Dry run

- **WHEN** the user runs `spinloop serve --dry-run`
- **THEN** the resolved command is printed and no server starts

#### Scenario: Engine not installed

- **WHEN** the selected engine's binary cannot be found
- **THEN** the error suggests installing that engine, not another one

#### Scenario: A piped run forwards the engine's output

- **WHEN** `spinloop serve` runs with its stdout piped or redirected
- **THEN** the engine's stdout and stderr are forwarded to serve's own, no
  view opens, and the printed command stays on stdout

### Requirement: Control API flag

`spinloop serve` SHALL accept `-a`/`--api` to expose the control API over the
foreground engine, as defined by the `daemon-api` capability. Serve SHALL
remain a foreground command with no daemon flag — long-lived supervision is
`spinloop daemon`'s job. The flag SHALL change only whether the control API
listens: the foreground behaviour — the view on a terminal, stdio forwarding
off one — is the same with and without it.

#### Scenario: Plain serve is unchanged

- **WHEN** the user runs `spinloop serve` without `--api`, with output not on
  a terminal
- **THEN** the engine runs in the foreground with stdio forwarded, exactly as
  before

#### Scenario: Serve with the API stays foreground

- **WHEN** the user runs `spinloop serve -a`
- **THEN** the engine runs in the foreground with the control API listening
  beside it, and the foreground behaviour is the same as without the flag

## ADDED Requirements

### Requirement: The serve view

`spinloop serve` on a terminal SHALL open a full-screen view of the engine it
is running: the engine's metrics, the engine's log, and a line naming the keys
the view answers to, in the three-section layout of the fleet dashboard's node
detail screen. The view SHALL need a terminal to draw on: a run whose output
is not a terminal SHALL NOT open it, and `--dry-run` SHALL open it neither,
printing the command without launching as it does today.

The view's metrics section SHALL show the same facts, in the same wording,
that the dashboard's detail screen and the metrics formats show for the same
engine — state, what is served, last active, the resource series, and the
token and request counters — read from the daemon the serve process runs
in-process rather than over the network, refreshed on the dashboard's own
local cadence. A reading the view could not renew SHALL be shown with its age
rather than drawn identically to one just read.

The view SHALL draw every resource series the reading carries in both formats
at once: the gauge of the current reading and, beneath it, the bar of the
retained history — the same series and labelling the bar and gauge formats
use, with the bar format's no-history rule where a series has no history to
draw, so a series with none carries its gauge alone.

#### Scenario: The view opens on a terminal

- **WHEN** the user runs `spinloop serve` at an interactive terminal
- **THEN** the view opens showing the engine's metrics above its log, with a
  line naming the keys the view answers to

#### Scenario: A piped run gets no view

- **WHEN** `spinloop serve` runs with its stdout piped or redirected
- **THEN** no view opens and the engine's output is forwarded to serve's own

#### Scenario: The metrics match the detail screen

- **WHEN** the view is open on a running engine
- **THEN** its metrics section shows the same state, serving facts, last
  active, resource series and counters the dashboard's detail screen shows
  for the same engine, in full rather than clipped

#### Scenario: Every series is drawn in both formats

- **WHEN** the reading carries a CPU series with a retained history
- **THEN** the view draws the series' gauge of the current reading and,
  beneath it, its bar of the retained history

#### Scenario: A series with no history carries its gauge alone

- **WHEN** a series has no retained history to draw
- **THEN** the view draws its gauge of the current reading only

### Requirement: The serve view's log

The view's log section SHALL show the engine's log, tailing and following it
the same way the dashboard's detail view follows its node's log: new output
appears while the view is open without the operator asking for it. An engine
that has written nothing yet SHALL show a waiting note, not an empty pane.

The up and down arrow keys SHALL scroll the log pane: up moves the visible
window one line towards the oldest retained line, down one line towards the
newest; a press at either end SHALL leave the window where it is. Page up and
page down SHALL move the window by the pane's height. While the window is on
the newest line, new output SHALL appear in the pane as it is written; while
the window is scrolled away from it, new output SHALL be retained and the pane
SHALL stay where the operator put it until they scroll back to the newest
line, where it sticks again.

The operator SHALL be able to pause and resume the log's follow from the
keyboard, independently of the rest of the view: while paused, the pane SHALL
stop picking up new output, and the view SHALL show whether the log is
following or paused. Nothing written while paused is lost: resuming SHALL
fetch and show whatever the engine wrote in the meantime, and pausing SHALL
not affect the metrics section's own refresh.

#### Scenario: The log pane follows new output

- **WHEN** the engine writes to its log while the view is open and the window
  is on the newest line
- **THEN** the new lines appear in the log section without the operator
  pressing any key

#### Scenario: Scrolling the log

- **WHEN** the operator presses the up arrow
- **THEN** the window moves one line towards the older output, and the down
  arrow moves it back, line for line, until it is on the newest line again
  and sticks to it

#### Scenario: Paging the log

- **WHEN** the operator presses page up
- **THEN** the window moves by the pane's height towards the older output,
  clamped at the oldest retained line

#### Scenario: An engine that has written nothing yet

- **WHEN** the view is open and the engine has written no log output
- **THEN** the log section shows a waiting note, not an empty pane

#### Scenario: Pausing the log

- **WHEN** the operator pauses the log's follow
- **THEN** the pane stops picking up new output and the view shows that the
  log is paused, while the metrics section keeps refreshing on its own cadence

#### Scenario: Resuming the log

- **WHEN** the operator resumes a paused log
- **THEN** whatever the engine wrote while paused appears in the pane, and
  new output continues to appear as it is written

### Requirement: The serve view's keys and exit

The view SHALL name its keys on screen and offer only the ones that would do
something in it: the up and down arrows and page up and page down scroll the
log, `f` pauses and resumes the log's follow, and `q` or Ctrl+C leaves. The
view SHALL NOT offer start, stop, keep or abort keys: the engine is serve's
own — starting is serve's job, and stopping it is what leaving does.

`q` and Ctrl+C SHALL stop the engine — gracefully, escalating as a stop does
elsewhere — and exit serve. When the engine exits on its own, whatever the
cause, the view SHALL close and serve SHALL exit with the engine's exit
status, exactly as a foreground serve does today.

#### Scenario: The key help names only live keys

- **WHEN** the view draws its key help line
- **THEN** it names the scroll, follow and quit keys, and nothing the view
  cannot do

#### Scenario: Quitting stops the engine

- **WHEN** the operator presses q or Ctrl+C
- **THEN** the engine is stopped and serve exits

#### Scenario: The engine's own exit closes the view

- **WHEN** the engine process exits while the view is open
- **THEN** the view closes and serve exits with the engine's exit status

### Requirement: Engine output capture under the serve view

When serve runs the view, the engine's stdout and stderr SHALL be captured to
the daemon's state-dir engine log — the same file `spinloop daemon` writes —
rather than forwarded to serve's stdio. The capture SHALL hold the engine's
whole output from its first line, so the log the view tails is complete, and
the log's path SHALL be the one the control API's status reports. With
`--api`, the API's log endpoint SHALL serve the captured file rather than
reporting the log missing.

#### Scenario: The engine's output lands in the log

- **WHEN** the engine writes to stdout or stderr while the view is open
- **THEN** the output is appended to the engine log file named in the
  daemon's status, and appears in the view's log section

#### Scenario: serve --api's log endpoint serves the capture

- **WHEN** `spinloop serve --api` runs under the view and a client asks its
  log endpoint for the engine log
- **THEN** the reply carries the engine's output, not the missing-log answer

#### Scenario: Off the terminal nothing is captured

- **WHEN** `spinloop serve` runs with its output not on a terminal
- **THEN** the engine's output is forwarded to serve's stdio and the run
  writes no engine log file

### Requirement: The view's metrics sampling

For an engine that exposes a metrics endpoint, a view run SHALL switch that
endpoint on before the engine starts — the same switch a supervised engine
gets — so the counters and the history the bars draw are the engine's own.
The daemon's activity and history sampling SHALL run for the life of the view,
the same sampling a running engine gets under the daemon, so the bars have a
retained history to draw. An engine with no metrics endpoint SHALL run with no
added switch, and its view SHALL draw the host's series — CPU, RAM and the
GPUs — as any node does.

#### Scenario: A metrics-capable engine is switched on

- **WHEN** `spinloop serve` opens the view for an engine that has a metrics
  endpoint
- **THEN** the engine is launched with its metrics endpoint on, the same way
  a supervised engine is

#### Scenario: History accrues for the bars

- **WHEN** the engine has been running for several sampler ticks under the
  view
- **THEN** the bars draw the retained history of the ticks so far

#### Scenario: An engine with no metrics endpoint

- **WHEN** `spinloop serve` opens the view for an engine that exposes no
  metrics endpoint
- **THEN** the engine is launched without any added switch, and the view
  draws the host's CPU, RAM and GPU series
