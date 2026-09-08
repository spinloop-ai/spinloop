## REMOVED Requirements

### Requirement: Bar format is default

Bar format was the default output when no `--format` flag was specified.

**Reason**: The default moves to the gauge format, which draws the current reading — the question an operator asks most often when watching an engine — while the bar's history stays one flag away.

**Migration**: The previous default display is available as `spinloop remote metrics --format=bar`.

## ADDED Requirements

### Requirement: Gauge format is default

The system SHALL use gauge format as the default output when no `--format` flag is specified.

#### Scenario: Default format is gauge

- **WHEN** the user runs `spinloop remote metrics` without `--format`
- **THEN** the output is in gauge format

## MODIFIED Requirements

### Requirement: Bar draws the retained history

Bar format SHALL draw each series from the history the on-instance daemon retains: readings taken at the sampler's cadence while the engine ran, covering at most the last 10 minutes. The sparkline SHALL show every sample the window holds, downsampled to the draw width where the window holds more samples than the width allows; downsampled points SHALL preserve the window's extremes rather than averaging them away.

The format SHALL be usable in one-shot mode: the history comes from the daemon, not from the command's own polling, so `--format=bar` without `--watch` draws the same window `--watch` would.

Where the daemon reports no history — a daemon that predates the feature, or an engine with no reading yet — bar format SHALL draw each series from the current reading alone, in the gauge's filled style, so a bar format that is asked for still shows the current level and a pre-history daemon renders what it can.

#### Scenario: One-shot bar shows the daemon's window

- **WHEN** the user runs `spinloop remote metrics --format=bar` without `--watch` against an engine that has been running
- **THEN** the output shows each series as a sparkline covering up to the last 10 minutes of the daemon's retained samples

#### Scenario: More samples than width are downsampled

- **WHEN** the retained window holds more samples than the draw width
- **THEN** the sparkline shows one glyph per column of the width, and a spike inside a downsampled range is still visible rather than smoothed away

#### Scenario: No history falls back to the gauge drawing

- **WHEN** the user runs `spinloop remote metrics --format=bar` against a daemon that reports no history
- **THEN** each series is drawn from the current reading in the gauge's filled style
