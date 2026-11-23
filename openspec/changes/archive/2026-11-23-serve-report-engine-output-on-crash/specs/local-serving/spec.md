# Spec Delta

## MODIFIED Requirements

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

When the engine exits unprompted with a failure, serve SHALL write the tail of
the captured engine log to stderr before it exits, under a header naming the
engine log's path, so the engine's own account of why it died outlives the
closed view. A stop on request and a clean exit SHALL print nothing extra. The
written tail SHALL be bounded to a fixed budget, and an engine log that is
empty or unreadable SHALL be said plainly rather than left as silence.

#### Scenario: The key help names only live keys

- **WHEN** the view draws its key help line
- **THEN** it names the scroll, follow, wrap and quit keys, and nothing the
  view cannot do

#### Scenario: Quitting stops the engine

- **WHEN** the operator presses q or Ctrl+C
- **THEN** the engine is stopped and serve exits, with no engine log tail
  written

#### Scenario: The engine's own exit closes the view

- **WHEN** the engine process exits while the view is open
- **THEN** the view closes and serve exits with the engine's exit status

#### Scenario: A crashed engine's output survives the view

- **WHEN** the engine exits unprompted with a failure while the view is open
- **THEN** serve writes the tail of the captured engine log to stderr before
  it exits, under a header naming the engine log's path, and still exits with
  the engine's exit status

#### Scenario: A clean exit writes nothing extra

- **WHEN** the engine exits successfully on its own while the view is open
- **THEN** serve writes no engine log tail and exits successfully
