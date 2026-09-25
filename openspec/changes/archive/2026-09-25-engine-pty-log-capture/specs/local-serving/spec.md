## MODIFIED Requirements

### Requirement: Engine output capture under the serve view

When serve runs the view, the engine's stdout and stderr SHALL be captured to
the daemon's state-dir engine log — the same file `spinloop daemon` writes —
rather than forwarded to serve's stdio. The capture SHALL hold the engine's
whole output from its first line, so the log the view tails is complete, and
the log's path SHALL be the one the control API's status reports. With
`--api`, the API's log endpoint SHALL serve the captured file rather than
reporting the log missing.

The engine's stdout SHALL be presented to the engine as a pseudo-terminal,
and the captured terminal output SHALL be normalised into log lines the same
way the daemon's engine log capture does: terminal escape sequences SHALL NOT
reach the log file, a line the engine redraws in place — a download progress
bar among them — SHALL be recorded as its state, the first and final state
always and any further distinct state at most once per fixed interval, and
the engine's stderr SHALL be written to the log file directly. A model
download's progress SHALL therefore appear in the view's log section while
the download runs, rather than the log showing the engine come up and then
silence.

#### Scenario: The engine's output lands in the log

- **WHEN** the engine writes to stdout or stderr while the view is open
- **THEN** the output is appended to the engine log file named in the
  daemon's status, and appears in the view's log section

#### Scenario: A model download's progress is shown

- **WHEN** the engine downloads a model while the view is open, printing its
  progress to stdout only because stdout is a terminal
- **THEN** the view's log section shows the download's progress as it runs,
  not silence

#### Scenario: serve --api's log endpoint serves the capture

- **WHEN** `spinloop serve --api` runs under the view and a client asks its
  log endpoint for the engine log
- **THEN** the reply carries the engine's output, not the missing-log answer

#### Scenario: Off the terminal nothing is captured

- **WHEN** `spinloop serve` runs with its output not on a terminal
- **THEN** the engine's output is forwarded to serve's stdio and the run
  writes no engine log file
