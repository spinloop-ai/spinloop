## Why

When an engine's output is captured to the engine log file — `spinloop daemon`
always, `spinloop serve` on a terminal — the engine's stdout is a file, not a
terminal. Engines that gate their terminal-only output on stdout being a
terminal therefore produce none of it: most notably llama.cpp prints its model
download progress bar only when `isatty(stdout)` holds, so a `spinloop serve`
run that fetches a model shows the server coming up and then silence while
several gigabytes are fetched. The model is downloading, but the user has no
way to know. The engine's HTTP API cannot fill the gap either, because the
download happens before the server starts listening.

## What Changes

- When the engine's output is captured to the engine log file, the engine's
  stdout is connected to a pseudo-terminal instead of the log file, so an
  engine that gates terminal output on a terminal — a download progress bar
  among them — produces it.
- The captured terminal output is normalised into log lines: terminal escapes
  and in-place line editing never reach the log file, and a line the engine
  redraws in place (a progress bar) is recorded as its state — the first
  state, the final state, and each further distinct state at most once per
  fixed interval — so a long download leaves a legible progression rather than
  thousands of duplicate lines or a bar frozen at its first frame.
- The engine's stderr keeps going to the log file directly, exactly as before.
- Where a pseudo-terminal cannot be allocated (no platform support), capture
  falls back to today's direct file write — no worse than the status quo.
- Off the terminal, nothing changes: a `spinloop serve` run whose stdout is
  not a terminal still forwards the engine's stdio untouched, and `serve --api`
  off a terminal still forwards as it does.

## Capabilities

### New Capabilities

(none — the capture behaviour already has homes in two existing capabilities)

### Modified Capabilities

- `serve-daemon`: the "Engine log capture" requirement — the engine's stdout is
  presented to the engine as a pseudo-terminal and the captured output is
  normalised to clean log lines, redrawing lines recorded as their state.
- `local-serving`: the "Engine output capture under the serve view" requirement
  — the same capture rule for the serve view's engine log, so the view's log
  section shows a download's progress while it runs.

## Impact

- `internal/daemon` — the supervisor gains the pseudo-terminal attach for a
  captured engine's stdout and the normaliser that turns the terminal stream
  into log lines; both sit beside the existing capture in `Start`.
- New dependency: a pseudo-terminal package (Unix only), imported from a
  build-tagged file so the Windows build compiles and runs on the fallback.
- Engine log files gain progress lines and carry no escape sequences; every
  consumer of the log (the serve view, the control API's `/v1/logs`, the fleet
  dashboard and `fleet logs`) sees the normalised lines with no contract
  change.
