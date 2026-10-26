# Spec Delta

## ADDED Requirements

### Requirement: Generation speed row

Where a reading carries a current generation speed or its retained history carries speed points, every surface that draws the resource series SHALL additionally draw a `Speed` row beneath them: a left-aligned label, the current figure right-aligned to two decimal places with a `t/s` suffix, and, where history is drawn, a sparkline of the retained speed points to the right of the figure.

The speed sparkline SHALL scale its glyph heights against the highest value drawn in the window — the window's high watermark maps to the sparkline's heaviest drawn grade, and no value draws the full block — so the row reads relative to itself, a steady engine filling its row and a doubled burst standing out against it. Every point SHALL appear in the terminal's default colour whatever its height: the 80/90% thresholds belong to utilisation percentages, and a generation rate has no ceiling to cross.

Where speed exists but the surface draws no history — the gauge format, the table format, a gauge-mode dashboard tile — the row SHALL carry the figure alone. Where neither the current reading nor the history carries a speed, the row SHALL be omitted entirely.

#### Scenario: Speed row with its sparkline

- **WHEN** bar format renders a reading whose history carries speed points
- **THEN** the output includes a `Speed` row whose figure is the current
  figure in t/s and whose glyphs are the retained speed points across the
  window

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
- **THEN** the output includes a `Speed` row carrying the t/s figure with no
  sparkline

#### Scenario: No speed, no row

- **WHEN** a reading's current figures and retained history carry no speed —
  an engine that reports none, or a daemon that predates the figure
- **THEN** the output draws no `Speed` row

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
