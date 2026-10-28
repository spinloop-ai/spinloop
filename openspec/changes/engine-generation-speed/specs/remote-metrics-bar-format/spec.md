# Spec Delta

## ADDED Requirements

### Requirement: Generation speed row

Where a reading carries a current generation speed or its retained history carries speed points — an engine that reports the figure, whatever it is doing this second — every surface that draws the resource series SHALL additionally draw a `Speed` row beneath them. The figure is the current reading to two decimal places with a `t/s` suffix; where it has lapsed between refreshes, the row SHALL read `0.00 t/s` — a stopped or quiet engine is generating at zero, and a row that kept showing the last burst's rate would be answering a question the engine is no longer asking. The row is omitted entirely only for an engine that reports no speed at all.

The speed sparkline SHALL place each point at its own sample's place on the window's time axis, and a sample without a rate SHALL draw blank: as an engine falls quiet, its sampling continues, and each quiet tick slides the last burst one column toward the past and leaves its own blank at the row's right edge — so the trend's now agrees with the figure's zero instead of holding the last burst against the edge as if it were still being generated. (A stopped engine's history froze at the stop, so its burst legitimately rests at the edge of a row whose figure reads zero: the row tells what the engine was doing until it stopped.)

The row shall share its columns with the rows above it, not cut its own: the figure starts where the other rows' graphs start — in the gauge formats in place of the gauge the rate has no ceiling to fill, in the combined view inside a fixed field spanning the gauge and percentage halves so the digits' first character holds the graph column whatever their count — and where history is drawn the sparkline SHALL start in the same column as every other sparkline and draw at the same width.

The speed sparkline SHALL scale its glyph heights against the highest value drawn in the window — the window's high watermark maps to the sparkline's heaviest drawn grade, and no value draws the full block — so the row reads relative to itself, a steady engine filling its row and a doubled burst standing out against it. Every point SHALL appear in the terminal's default colour whatever its height: the 80/90% thresholds belong to utilisation percentages, and a generation rate has no ceiling to cross.

Where speed exists but the surface draws no history — the gauge format, the table format, a gauge-mode dashboard tile — the row SHALL carry the figure alone, at the column the gauges start.

#### Scenario: Speed row with its sparkline

- **WHEN** bar format renders a reading whose history carries speed points
- **THEN** the output includes a `Speed` row whose glyphs are the retained
  speed points starting in the same column as the other sparklines and
  whose trailing figure is the current figure in t/s

#### Scenario: The figure stands where the graphs start

- **WHEN** the combined view renders a reading with a current generation
  speed
- **THEN** the `Speed` row's figure starts at the column the gauges start,
  and its sparkline at the column the other sparklines start, so a rate
  growing from "5.06 t/s" to "23.45 t/s" keeps both columns fixed

#### Scenario: A lapsed figure reads zero on an engine that reports speed

- **WHEN** a watched engine has printed no timing figure for the last
  sampling interval while its window still holds speed points
- **THEN** the `Speed` row remains drawn, its figure reads 0.00 t/s, and
  the sparkline draws the quiet interval blank at its right edge, the
  retained burst slid that far toward the past

#### Scenario: The window's fastest point draws the heaviest glyph

- **WHEN** a speed history's window holds values from 2.0 to 10.0 t/s
- **THEN** the 10.0 point draws the heaviest grade the sparkline draws, the
  2.0 point draws one far lighter, and no glyph in any row of the output is
  the full block

#### Scenario: Speed points take no threshold colour

- **WHEN** a speed sparkline holds a point far above the 80/90% thresholds'
  territory at the window's maximum
- **THEN** that glyph appears in the terminal's default colour, and so does
  every other speed glyph

#### Scenario: A figure with no history still draws

- **WHEN** the gauge format renders a running reading with a current speed and
  no retained speed points
- **THEN** the output includes a `Speed` row carrying the t/s figure at the
  column the gauges start, with no sparkline

#### Scenario: No speed, no row

- **WHEN** a reading's current figures and retained history carry no speed —
  an engine that reports none, or a daemon that predates the figure
- **THEN** the output draws no `Speed` row

### Requirement: Label column fits the labels

The label column shared by the rows of a metrics block — the resource
series, the generation speed, and the active figure drawn beside them —
SHALL be as wide as the longest label the block draws and no wider: seven
columns on hosts with at most one GPU (the longest label there is
"GPU mem"), widening by the index the labels carry where the host names
more than one GPU. The column was sized for the retired "GPU util" label;
the block keeps one column per fleet rather than one per reading, so the
blocks of different hosts in one `fleet metrics` output read as one table.

#### Scenario: One GPU sets the column

- **WHEN** `spinloop metrics --format=bar` renders a host with a single GPU
- **THEN** the resource rows' graphs start two columns nearer the labels
  than they did when the utilisation series was labelled "GPU util", and
  every row of the block starts its figure in the same column

#### Scenario: Many GPUs widen it

- **WHEN** the same command renders a host whose readings name GPU 0 and
  GPU 1
- **THEN** the column widens for the "GPU 0 mem" label and every row of the
  block, CPU included, still starts its graph in the same column

## MODIFIED Requirements

### Requirement: Bar format output

The system SHALL support a `--format=bar` option that renders each resource series as a sparkline drawn from the history the on-instance daemon retains: a left-aligned label, one glyph per sample, and the latest value as a right-aligned percentage. The glyphs SHALL be Unicode block elements of one grade per utilisation level, so a series reads as a line of bars across the window. The series drawn SHALL be the same set the gauge format draws: CPU, RAM, each GPU's utilisation — labelled "GPU", or "GPU N" where the host has more than one — and each GPU's memory where that GPU's current reading or retained history reports a memory total — labelled "GPU mem"/"GPU N mem" — a GPU that never reports one drawing no memory series, with the same per-GPU labelling throughout.

#### Scenario: Bar format displays CPU utilization

- **WHEN** the user runs `spinloop cloud metrics --format=bar` with a running instance that has CPU data and a retained history
- **THEN** the output includes a row labelled "CPU" whose glyphs are the sampled CPU utilisation across the window and whose trailing figure is the latest sample's percentage

#### Scenario: Bar format displays RAM utilization

- **WHEN** the user runs `spinloop cloud metrics --format=bar` with a running instance that has memory data
- **THEN** the output includes a row labelled "RAM" whose glyphs are the sampled used/total memory ratio across the window and whose trailing figure is the latest ratio

#### Scenario: Bar format displays GPU utilization

- **WHEN** the user runs `spinloop cloud metrics --format=bar` with a running instance that has GPU data whose readings carry a memory total
- **THEN** the output includes rows labelled "GPU" and "GPU mem" (or "GPU N"/"GPU N mem" for multiple GPUs), each drawn from the retained history

#### Scenario: Bar format omits a GPU memory series with no total

- **WHEN** the user runs `spinloop cloud metrics --format=bar` with GPU data whose current reading and retained history report no memory total
- **THEN** the output includes "GPU" and no "GPU mem" row

#### Scenario: Bar format header line

- **WHEN** the user runs `spinloop cloud metrics --format=bar` with a running instance
- **THEN** the first line shows the environment, state, instance type, and model ID separated by double spaces

### Requirement: Gauge format

The system SHALL support a `--format=gauge` option that renders each resource series as a horizontal progress gauge: a left-aligned label, a filled portion using block characters, an unfilled portion using light shade characters, and a right-aligned percentage value. The gauge draws the current reading only — it carries no history. The series drawn SHALL be CPU, RAM, each GPU's utilisation — labelled "GPU", or "GPU N" where the host has more than one — and each GPU's memory where the current reading reports a memory total — labelled "GPU mem"/"GPU N mem" — a GPU that reports none drawing no memory gauge, with the same labels the bar format uses. The gauge fill SHALL be colour-coded on the bar format's thresholds: green for values at or below 80%, yellow for values from 80% to 90%, and red for values above 90%, with the colour reset after the filled portion so the unfilled characters and percentage appear in the terminal's default colour.

#### Scenario: Gauge format displays CPU utilization

- **WHEN** the user runs `spinloop cloud metrics --format=gauge` with a running instance that has CPU data
- **THEN** the output includes a gauge labelled "CPU" with filled and unfilled segments proportional to the current utilization

#### Scenario: Gauge format displays GPU utilisation

- **WHEN** the user runs `spinloop cloud metrics --format=gauge` with a running instance that has GPU data whose readings carry a memory total
- **THEN** the output includes gauges labelled "GPU" and "GPU mem" (or "GPU N"/"GPU N mem" for multiple GPUs)

#### Scenario: Gauge omits a GPU memory gauge with no total

- **WHEN** the user runs `spinloop cloud metrics --format=gauge` with a GPU whose current reading reports no memory total
- **THEN** the output includes "GPU" and no "GPU mem" gauge

#### Scenario: Gauge colours the fill

- **WHEN** a gauge's current value is 95%
- **THEN** its filled segment appears in red, and its unfilled segment and percentage appear in the terminal's default colour
