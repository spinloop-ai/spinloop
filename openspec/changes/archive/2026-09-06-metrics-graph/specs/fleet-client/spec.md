## MODIFIED Requirements

### Requirement: Fleet metrics

`spinloop fleet metrics` SHALL query every node's metrics endpoint and render
each node's engine and system metrics using the same bar, gauge, table, and
json formats `spinloop remote metrics` provides, selected by `--format`.
Unreachable nodes SHALL be reported as in status rather than omitted. The
command SHALL support a `--watch`/`-w` mode that refreshes on an interval,
clearing and redrawing the screen in place with no scrollback accumulation,
and exiting cleanly on interrupt.

#### Scenario: Bar format per node

- **WHEN** `spinloop fleet metrics` runs without `--format`
- **THEN** each reachable node's metrics render in bar format under its name

#### Scenario: Gauge format per node

- **WHEN** `spinloop fleet metrics --format=gauge` runs
- **THEN** each reachable node's resource series render in gauge format under
  its name

#### Scenario: JSON aggregates the fleet

- **WHEN** `spinloop fleet metrics --format=json` runs
- **THEN** the output is valid JSON keyed or labelled by node, including
  unreachable nodes with their error

#### Scenario: Watch redraws in place

- **WHEN** `spinloop fleet metrics --watch` runs
- **THEN** each refresh clears the screen and redraws the fleet, and Ctrl+C
  exits cleanly

### Requirement: Dashboard panels show the node's metrics

Each panel SHALL show, for a node that answered the last completed refresh, the
same facts the bar format of `fleet metrics` renders for that node: its state,
what it serves (runner and model when known), how long since it last did work
(with the same labelling rules as the rest of the fleet surfaces), its resource
usage, and its token and request counters. A panel SHALL draw the node's
resource series in the board's current format — bar by default — from the
history the node's daemon reports, falling back per the bar format's no-history
rule where it reports none. A panel SHALL show the answer of the last completed
refresh for that node — not a mix of refreshes and not a stale bar with a fresh
outcome.

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

## ADDED Requirements

### Requirement: Dashboard format toggle

The dashboard SHALL provide a key, `g`, that toggles the resource series of
every panel between bar and gauge. The board SHALL open in bar. The toggle
SHALL be board-wide — one format for every panel — rather than per node, and
the key help line SHALL name it.

#### Scenario: Pressing the key switches every panel

- **WHEN** the operator presses `g` on the grid
- **THEN** every panel's resource series redraws in the other format, and
  pressing `g` again returns them

#### Scenario: The board opens in bar

- **WHEN** the dashboard opens
- **THEN** the panels draw the resource series in bar format

#### Scenario: The key help names the toggle

- **WHEN** the dashboard draws its key help line
- **THEN** it names `g` as the format toggle
