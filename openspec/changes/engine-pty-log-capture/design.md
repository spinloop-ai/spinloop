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
stderr is the engine's log lines (llama.cpp's `common_log` routes them to
stderr), and — more importantly — a file write blocks the engine only on a
full disk, while a pseudo-terminal blocks a writer that outpaces its reader.
The pump drains continuously, but liveness should not depend on a reader
being fast: stderr keeps the engine's only truly unblocked output path.

Because the engine's stdout is now a terminal, the engine is told
`NO_COLOR=1` in its environment. An engine that colours by terminal
presence — and colours stderr by whether *stdout* is a terminal, which is
llama.cpp's rule — would otherwise write escapes to the log through the
stderr path, the one the normaliser never sees. A download bar draws no
colour, so nothing is lost; and the forwarding path (no log file) sets
nothing, so a foreground engine on a real terminal keeps its colour.

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

### 4. The normaliser: a column of lines, and the states of the ones redrawn

A small state machine turns the pseudo-terminal stream into log lines. It
keeps the terminal's lines as a column, top down, the way the screen holds
them; it is a line model, not a screen emulator:

- The terminal's CRLF becomes the log's LF; a newline (CRLF or bare) commits
  the line the drawing is on — a plain line as written, a redrawing line as
  its final state — and moves the drawing to the line below. A committed
  line stays in the column: a cursor move may come back to it, and a line
  touched after its commit is recorded again, as the terminal shows it.
- A carriage return that is not a line ending starts a **redraw** of the
  line the engine is drawing: what follows is a new state of that line,
  replacing the state, not appending to it. A state is complete when the
  engine moves on — a new state, a newline, an erase, or the end of the
  stream — so it is never recorded half-drawn.
- Cursor moves do not end a line: up, down and home move the drawing between
  the column's lines. That is how the engines redraw a bar in place —
  llama.cpp's `ProgressBar` parks its cursor on an anchor line and, for
  every update, moves up to the bar's line, draws the state, and moves back
  down —
  so a bar's line is redrawn in place, and a multi-file download's several
  bars sit on separate lines, each recording its own states. A move lands
  the drawing on the line it arrives at: whatever the engine writes there
  next begins a new state of the line — the terminal's rewrite, not an
  append — so a home or an up onto a committed line corrects the line
  rather than extending its content. Committing on a cursor move was the
  first design, and the first real engine run showed why it is wrong: the
  engine's up-down dance is part of each redraw, so a commit per move
  records every state as a "final" one and resets the
  dedup — the whole download, state for state, in the log.
- Erase sequences shape the state the way they shape the terminal's line: a
  whole-line erase (2K, J) ends the state drawn on the line and clears what
  it holds; an erase to the end of the line (a bare K) erases from the
  cursor, which sits at the end of the state, and changes nothing.
- A redrawing line is recorded as its state under a frequency rule: the
  first state is always recorded, the final state always, and a further
  distinct state at most once per fixed interval (2 s). Identical states are
  never repeated. A 2-second interval bounds a chatty bar — 1,000 updates a
  download makes — to a legible run of lines, and sits under the view's own
  3 s poll so the pane sees fresh states as they land.
- Because a final state can arrive and then no byte ever follow (the
  download finishes and the engine goes quiet on stdout while it loads, the
  cursor on its anchor), the tick at the interval's half-rate records a
  pending state whose time has come on every line of the column; dedup makes
  the tick a no-op once nothing is pending. End of stream commits the whole
  column, each line's final state unthrottled.
- Every other escape sequence is dropped, and one that is not part of the
  redraw's own unit — the erase and the cursor moves — ends the state drawn
  before it. No escape ever reaches the log.

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
  clock: unit tests feed it captured-terminal bytes — a progress run, the
  bar the engines actually draw (the cursor up-down around the anchor),
  interleaved bars, a cursor move onto a committed line, the escape
  sequences the engines may send, an erase, CRLF, a pending final state at
  end of stream —
  and assert the exact recorded lines and the frequency rule's boundaries.
  The pump's end-of-stream drain is tested with the error arriving with,
  and before, the last bytes.
- The supervisor test uses a shell one-liner as the engine: it prints a plain
  line, prints a line only when `[ -t 1 ]` holds, and redraws a progress
  line with raw `\r` bytes. The assertions read the log file: the TTY-gated
  line is present (the pseudo-terminal worked), the redraws recorded as
  states without escapes, the plain line untouched, and the engine carried
  `NO_COLOR` (the forwarding test asserts the opposite: the engine it
  forwards was told nothing).
- With the pseudo-terminal's opening stood in for with a failure, the
  fallback test asserts the spec's no-pseudo-terminal scenario: the
  engine's stdout in the log as written, and the engine told its output is
  going to a file the same way.

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
