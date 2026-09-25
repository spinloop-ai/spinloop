## 1. Pseudo-terminal attach

- [x] 1.1 Add the `creack/pty` dependency (go.mod/go.sum)
- [x] 1.2 `internal/daemon/pty_unix.go` (`//go:build !windows`): open a pseudo-terminal at the fixed window size for a captured engine's stdout, returning the master to read and the close/handoff the supervisor needs
- [x] 1.3 `internal/daemon/pty_windows.go` (`//go:build windows`): report a pseudo-terminal as unavailable so the supervisor falls back
- [x] 1.4 Wire the attach into `Supervisor.Start`'s capture branch: stdout to the pseudo-terminal, stderr to the log file as today, direct file write for stdout where the attach is unavailable, with a debug record when the fallback is taken

## 2. The normaliser

- [x] 2.1 `internal/daemon`: the line-state normaliser over the pseudo-terminal stream — CRLF to LF, plain lines committed on newline, `\r` redraws, erase and cursor-move handling, all other escapes dropped
- [x] 2.2 The frequency rule: first and final state of a redrawing line always recorded, a further distinct state at most once per the fixed interval, identical states never repeated, an injected clock
- [x] 2.3 The pending-state tick (records a final state after the stream goes quiet) and the end-of-stream final record

## 3. The pump and shutdown

- [x] 3.1 The pump goroutine: read the master, feed the normaliser, write its lines to the log file, always draining; end-of-stream handling per platform including draining bytes that ride with the error
- [x] 3.2 Shutdown order in `Start`'s wait goroutine: wait for the engine, end the pump's read, wait for the pump's final record, then close the log file
- [x] 3.3 Verify the forwarding path (empty `LogPath`) is byte-for-byte unchanged

## 4. Tests

- [x] 4.1 Normaliser unit tests: a progress run, an erase, a cursor move, CRLF, a pending final state at end of stream, and the frequency rule's boundaries, against exact recorded output
- [x] 4.2 Supervisor test with a shell-script engine: a TTY-gated line (`[ -t 1 ]`) appears in the log, a `\r`-redrawn line records as states without escapes, a plain line is untouched
- [x] 4.3 Full suite green with `-race -cover`, coverage at or above the 80% bar

## 5. Docs

- [x] 5.1 Maintainer internals: why the capture presents stdout as a pseudo-terminal and how the normaliser records redrawing lines, so the next reader does not "simplify" the fallback away
