# Spec Delta

## MODIFIED Requirements

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
at once, each series on one line: its gauge of the current reading and its
bar of the retained history side by side, the figure of the current reading
between the two halves — the same series and labelling the bar and gauge
formats use, with the bar format's no-history rule where a series has no
history to draw, so a series with none carries its gauge alone, its bar half
blank. The figure SHALL draw in a fixed-width field, so a reading of one, two
or three digits keeps the line the same width and the bar half in the same
column: a reading that crosses a digit boundary SHALL NOT shift the series'
bar.

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
- **THEN** the view draws the series on one line: its gauge of the current
  reading and its bar of the retained history, side by side

#### Scenario: A series with no history carries its gauge alone

- **WHEN** a series has no retained history to draw
- **THEN** the view draws its gauge of the current reading only, its bar
  half left blank

#### Scenario: A reading that crosses a digit boundary does not shift the bar

- **WHEN** a series' reading is 7, then 42, then 100, with the same retained
  history each time
- **THEN** the line is drawn the same width at each reading, with the bar
  half starting in the same column
