## MODIFIED Requirements

### Requirement: Engine log capture

The daemon SHALL write the supervised engine's stdout and stderr to a log
file rather than the daemon's own stdio, and SHALL report the log file's path
in its status so the user can find it.

The engine's stdout SHALL be presented to the engine as a pseudo-terminal
rather than the log file itself: an engine that gates terminal-only output on
stdout being a terminal — a model download's progress among them — SHALL
produce that output while supervised. The captured terminal output SHALL be
normalised into log lines: terminal escape sequences SHALL NOT reach the log
file, and a line the engine redraws in place — a download progress bar among
them — SHALL be recorded as its state, the first state and the final state
always and any further distinct state at most once per fixed interval, so a
long download leaves a legible progression rather than an unbounded run of
duplicates. The engine's stderr SHALL be written to the log file directly,
as before the pseudo-terminal existed. Where a pseudo-terminal cannot be
allocated, the engine's stdout SHALL be written to the log file directly
instead, no worse than the capture did before.

#### Scenario: Engine output lands in the log file

- **WHEN** a supervised engine writes to stdout or stderr
- **THEN** the output is appended to the engine log file named in the daemon's
  status

#### Scenario: Terminal-only output is captured

- **WHEN** a supervised engine writes to stdout only when stdout is a
  terminal, as a model download's progress does
- **THEN** that output appears in the engine log file

#### Scenario: A redrawing line is recorded as its state

- **WHEN** the engine redraws a line in place, as a download progress bar
  does
- **THEN** the log carries the line's first state, its final state, and each
  further distinct state at most once per the fixed interval, with no
  terminal escape sequences in any of the recorded states

#### Scenario: stderr is captured directly

- **WHEN** a supervised engine writes to stderr
- **THEN** the output is appended to the engine log file unaltered, as before
  the pseudo-terminal existed

#### Scenario: No pseudo-terminal available

- **WHEN** the platform cannot allocate a pseudo-terminal for the engine
- **THEN** the engine's stdout is written to the log file directly, and the
  engine runs and is supervised exactly as before
