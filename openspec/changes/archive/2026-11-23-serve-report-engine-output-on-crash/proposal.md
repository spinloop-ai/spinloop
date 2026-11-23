# Proposal

## Why

On a terminal, `spinloop serve` (and `spinloop up`, which runs serve's own body)
captures the engine's output to the daemon's state-dir engine log and shows it
only inside the serve view. When the engine fails at startup — a `PARAM` or
preset key the engine rejects, for instance — the view closes on the engine's
exit and the console is left with nothing but `Error: exit status 1`. The lines
that say *why* the engine died sit in `engine.log`, invisible to the operator
who just watched the command fail. Diagnosis means opening the log file by hand,
which defeats the point of a foreground command.

## What Changes

- When a view run's engine exits unprompted with a failure, serve writes the
  tail of the captured engine log to stderr before it exits, under a header
  naming the engine log's path.
- A clean exit and a stop on request (`q`, Ctrl+C, or the API) print nothing
  extra and keep today's exit status (success for a stop, the engine's status
  for a clean exit).
- The piped/redirected run is unchanged: it already forwards the engine's
  stdio live, so its failure output is already on the console.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `local-serving`: "The serve view's keys and exit" gains the rule that a
  crashed engine's captured output — the engine log's tail — is written to
  stderr when the view closes on the engine's own failed exit.

## Impact

- `cmd/spinloop/serve_view.go` — the post-view exit path reports the captured
  log tail when the run ended in a crash.
- Tests in `cmd/spinloop` pin the rule without a terminal, as the rest of the
  serve suite does.
- No daemon, control-API, or engine-table changes: the captured log and its
  bounded read (`daemon.ReadLog`) already exist and are reused.
