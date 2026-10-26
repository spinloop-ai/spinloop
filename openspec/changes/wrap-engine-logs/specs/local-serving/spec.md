# Spec Delta

## MODIFIED Requirements

### Requirement: The serve view's log

The view's log section SHALL show the engine's log, tailing and following it
the same way the dashboard's detail view follows its node's log: new output
appears while the view is open without the operator asking for it. An engine
that has written nothing yet SHALL show a waiting note, not an empty pane.

A log line longer than the pane's width SHALL wrap onto continuation rows at
the pane's left margin, so the whole line is readable within the pane;
wrapping SHALL be on by default. The operator SHALL be able to turn wrapping
off and back on from the keyboard; while wrapping is off a longer line SHALL
be cut at the pane's width as before. The view SHALL show whether wrapping is
on or off. When wrapping is on, the pane's rows are its window's lines
expanded to their wrapped rows, the newest rows at the pane's bottom.

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

#### Scenario: A line longer than the pane wraps

- **WHEN** the engine writes a line wider than the log pane
- **THEN** the line is drawn across as many rows as it needs, every row
  starting at the pane's left margin, with no part of the line lost

#### Scenario: Turning wrapping off and on

- **WHEN** the operator turns wrapping off
- **THEN** lines longer than the pane are cut at its width again, and the
  view shows that wrapping is off
- **AND** turning it back on wraps those lines again

#### Scenario: Scrolling a wrapped line

- **WHEN** the operator scrolls the pane by one line
- **THEN** the whole wrapped line — all of its rows — moves out of or into
  the pane as one line

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
log, `f` pauses and resumes the log's follow, `w` turns log wrapping on and
off, and `q` or Ctrl+C leaves. The view SHALL NOT offer start, stop, keep or
abort keys: the engine is serve's own — starting is serve's job, and stopping
it is what leaving does.

`q` and Ctrl+C SHALL stop the engine — gracefully, escalating as a stop does
elsewhere — and exit serve. When the engine exits on its own, whatever the
cause, the view SHALL close and serve SHALL exit with the engine's exit
status, exactly as a foreground serve does today.

#### Scenario: The key help names only live keys

- **WHEN** the view draws its key help line
- **THEN** it names the scroll, follow, wrap and quit keys, and nothing the
  view cannot do

#### Scenario: Quitting stops the engine

- **WHEN** the operator presses q or Ctrl+C
- **THEN** the engine is stopped and serve exits

#### Scenario: The engine's own exit closes the view

- **WHEN** the engine process exits while the view is open
- **THEN** the view closes and serve exits with the engine's exit status
