## Context

The supervisor (`internal/daemon.Supervisor.Start`) is the one place an
engine's output is captured: it points the engine's stdout and stderr at the
engine log file (daemon, and the serve view), or at its own stdio (a
`serve --api` run off a terminal). The engine log is a record — append-only,
read by `ReadLog` for the control API's `/v1/logs`, the serve view's log
section, the fleet dashboard and `fleet logs` — and `ReadLog` speaks whole
lines plus a byte cursor.

llama.cpp's download `ProgressBar` (`common/download.cpp`) prints nothing
when `isatty(stdout)` is false, so under the capture a model download is
silent. The server's HTTP model-status API (newer llama.cpp reports
`downloading` on `/v1/models`) cannot be scraped here: in this flow the
download happens in-process before the HTTP listener comes up.

## Goals / Non-Goals

**Goals:**

- A captured engine (daemon; serve under the view) gets its terminal-only
  stdout output — the model download's progress among them — into the engine
  log.
- The log stays a record of clean lines: no escape sequences, a redrawing
  line (a progress bar) recorded as a legible progression with a bounded
  line count, first and final state always present.
- Every consumer of the log benefits with no contract change: the serve
  view, `/v1/logs`, the fleet dashboard, `fleet logs`.

**Non-Goals:**

- No change to off-terminal `serve` (piped runs forward stdio untouched) or
  to `serve --api` off a terminal (stdio forwarding) — the engine there has
  whatever terminal, if any, the user gave it, exactly as before.
- No live in-place progress bar in the view: the log is a record and the
  view tails it. The progression is a series of recorded states, refreshed on
  the view's own poll cadence.
- No change to what the daemon runs, how it is driven, or the control API's
  contract.

## Decisions

### 1. The fix lives in `Supervisor.Start`, on the capture path only

`Start` already branches on `LogPath`: empty forwards to stdio, set writes to
the file. The pseudo-terminal attach happens in the file branch, so the
daemon and the serve view — the only two captured paths — share one
implementation, and the forwarding paths keep their exact current behaviour.
An engine's TTY-ness is a property of where its output goes, so the rule
follows the capture rather than the command.

Alternative considered: fixing it in each command (`runServeView`,
`runDaemonCommand`). Rejected — two call sites of the same capture drifting
is exactly how this class of bug appears.

### 2. Only stdout goes to the pseudo-terminal; stderr keeps its file

The engine's stderr writes straight to the log file as today. Two reasons:
stderr is plain log lines (llama.cpp's `LOG_*` prints unconditionally, no TTY
gate), and — more importantly — a file write blocks the engine only on a full
disk, while a pseudo-terminal blocks a writer that outpaces its reader. The
pump drains continuously, but liveness should not depend on a reader being
fast: stderr keeps the engine's only truly unblocked output path.

### 3. `creack/pty`, build-tagged, with a direct-file fallback

The pseudo-terminal is a small, well-worn dependency (pure Go over
`x/sys`, no cgo — the release builds run `CGO_ENABLED=0`). It is imported
only from a `//go:build !windows` file, with a Windows file returning
"unavailable": GoReleaser builds Windows, and there the capture falls back
to today's direct file write — the status quo, no worse. A PTY allocation
failure at runtime (a restricted environment) takes the same fallback, so
the engine always starts.

Alternative considered: raw `x/sys/unix.Openpty` to avoid a new dependency.
Rejected — the dependency buys the edge cases (winsize, fd handoff, the
master-read end-of-stream behaviour per platform) for less code; `x/sys` is
already in the module.

### 4. The normaliser: line states, not terminal emulation

A small state machine turns the pseudo-terminal stream into log lines. It is
a line model, not a screen:

- Bytes are normalised first: CRLF becomes LF (the terminal's output
  translation), then processed.
- A byte run without `\r` or `\n` is the line being written; `\n` commits it
  as a plain log line.
- `\r` starts a **redraw** of the current line: what follows is a new state
  of that line, not new output. Erase sequences (clear-to-end, clear-line)
  truncate the pending state; cursor moves (up/down) end the current
  redraw and begin a new one, so a multi-file download's several bars record
  as interleaved states rather than corrupting each other. Every other
  escape sequence is dropped: no escape ever reaches the log.
- A redrawing line is recorded as its state under a frequency rule: the
  first state is always recorded, the final state always, and a further
  distinct state at most once per fixed interval (2 s). Identical states are
  never repeated. A 2-second interval bounds a chatty bar — 1,000 updates a
  download makes — to a legible run of lines, and sits under the view's own
  3 s poll so the pane sees fresh states as they land.
- Because the final state can arrive and then no byte ever follow (the
  download finishes and the engine goes quiet on stdout while it loads), a
  tick at the interval's half-rate records a pending state whose time has
  come; dedup makes the tick a no-op once nothing is pending. End of stream
  records the pending state and the run is done.

The rule is engine-agnostic — it reads line states, never their content — so
it serves whatever bar an engine draws, now or later.

### 5. The pump owns the drain and the shutdown order

A goroutine reads the pseudo-terminal's master, feeds the normaliser, and
writes the normaliser's lines to the log file. It always drains — the
throttle delays log *writes*, never reads — so the engine can never wedge on
a full terminal buffer. On exit the ordering in `Start`'s wait goroutine is:
wait for the engine, close the master (which ends the read, draining any
bytes that ride with the end-of-stream error), wait for the pump to finish
its final record, and only then close the log file. The file is closed after
the pump's last write, never under it.

The pseudo-terminal opens at a fixed 80×50 window: wide enough that an
engine sizes its output for a normal terminal, narrow enough that recorded
lines stay compact for the pane and for log consumers.

### 6. Tests: a fake clock and a fake engine

- The normaliser is a plain function over a byte stream with an injected
  clock: unit tests feed it captured-terminal bytes (a progress run, an
  erase, a cursor move, CRLF, a pending final state at end of stream) and
  assert the exact recorded lines and the frequency rule's boundaries.
- The supervisor test uses a shell one-liner as the engine: it prints a plain
  line, prints a line only when `[ -t 1 ]` holds, and redraws a progress
  line with raw `\r` bytes. The assertions read the log file: the TTY-gated
  line is present (the pseudo-terminal worked), the redraws recorded as
  states without escapes, and the plain line untouched.

## Risks / Trade-offs

- [The platform or environment refuses a pseudo-terminal] → the direct-file
  fallback is the pre-change behaviour; the engine runs and is supervised
  exactly as before, and the failure is recorded at debug level.
- [An engine misbehaves on a terminal it was not asked to be one] → the
  engine's stdin stays `/dev/null` as today, so nothing it reads from stdin
  gets a surprise; its window is a fixed 80×50.
- [A download with many bars (a split model) records interleaved states] →
  each recorded line is a self-contained statement ("file, position,
  percent"), so the interleaving reads as a log rather than a broken bar.
- [The very last bytes of the final state could be lost where the platform
  signals end of stream abruptly (Linux's EIO)] → the read loop drains the
  bytes that arrive with the error before stopping, and the throttled
  records mean the final state is almost always already in the log by then.
- [The log gains lines it did not have before] → bounded by the frequency
  rule (a long download adds a couple of hundred lines, each legible), and
  the remote instance's size-based rotation already bounds the file.

## Migration Plan

No data or protocol migration: the engine log's path, format contract and
every consumer are unchanged; the file gains clean progress lines and no
escape sequences. Rollback is reverting the change — capture returns to the
direct file write.

## Open Questions

None — the platform fallback's exact wording in the debug record and the
winsize value are implementation details the spec does not pin.
