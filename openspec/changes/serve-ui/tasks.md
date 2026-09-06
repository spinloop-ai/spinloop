## 1. Combined bar-and-gauge metrics rendering

- [x] 1.1 Give `renderGauge` a width parameter in `cmd/spinloop/metrics_render.go`, updating its existing callers to pass today's 25-column width, and verify the existing metrics render tests pass unchanged
- [x] 1.2 Add the combined renderer (each series from `barSeriesList` as its gauge line of the current reading, then its sparkline line beneath where the history holds samples, the sparkline's label blank so the pair stacks, a history-less series gauge-only) and verify with table-driven tests covering: a series with history (both lines), a series with none (gauge alone), multi-GPU labelling, and the 80/90 colour thresholds on the gauge

## 2. Serve view model

- [x] 2.1 Create `cmd/spinloop/serve_view.go` with the model's state (the metrics reading and its time, the tailed log content, its byte offset, its line budget, the scroll position and the follow flag, the window size) and its `Init`/`Update`/`View` shape, and verify the package compiles and an initial-state test passes
- [x] 2.2 Add the metrics tick: a one-shot 2-second tick taking an in-process status-plus-metrics reading through an injected read function, a failed or stale reading leaving the last one in place with its age, and verify with tests that a reading replaces the prior one and a failed read keeps the last
- [x] 2.3 Add the log poll: a one-shot 3-second tick with a generation guard, reading through an injected log read with the daemon's log semantics (tail on first read, resume from the stored offset, `StaleOffset` resetting to the reply's `NextOffset`), appending and trimming to the line budget, and verify with tests for the first read's backlog, no duplicates across polls, a stale-offset resume, and a superseded poll being discarded
- [x] 2.4 Add the keys: `up`/`down` moving the scroll by one line, `pgup`/`pgdown` by the pane's height, clamped at the oldest retained line and the newest, the window sticking to the tail at the newest line and staying put while scrolled away, `f` pausing and resuming the follow, and `q`/Ctrl+C quitting, and verify with table-driven key tests over the scroll positions and the follow flag
- [x] 2.5 Add the engine-exited message delivered through the model's `send` seam (the dashboard's `prog.Send` door), quitting the program on it, and verify a test drives the message and asserts the quit

## 3. Serve view frame

- [x] 3.1 Draw the frame in `View()`: the title bar (screen "serve", the Spinloop's path and the log's state to the right), the metrics section (state with uptime, the serving line, the last-active line, the combined renderer, the token lines), the log section with the waiting note where there is no content yet, the dividers, and the footer key help naming exactly the scroll, follow and quit keys, reusing the detail screen's `dashTitleBar`, key-hint and clip helpers, and verify with render tests over a fixed model and terminal size
- [x] 3.2 Show the log's following/paused state in the title bar from the follow flag, and verify a render test for each state

## 4. `serve` wiring

- [x] 4.1 Gate the view on `term.IsTerminal` of stdout in `runServe` (never for `--dry-run`, never off the terminal), pass the narration writer through `buildServeArgv` so the command and preset/model lines go to stderr under the view and stay on stdout off it, and verify the existing serve tests pass plus a test that the gate is applied through an injectable check
- [x] 4.2 Pull the supervised-foreground construction out of `runServeForegroundAPI` into one helper — the supervisor, the daemon with its served name, scrape target, engine endpoint and refusing start stub, the metrics switch for an engine that has one, the sampler for the run's life, the signal relay, the graceful stop and the exit-status rule (a stop on request as success) — parameterised only by the supervisor's log target (the state-dir engine log, or empty for stdio forwarding) and whether the control API listener comes up, and point the existing non-terminal `--api` path at it, and verify the existing foreground-serve tests pass unchanged
- [x] 4.3 Build the view run on that helper — log target the state-dir engine log, listener per `--api`, the engine started before the program opens so a missing binary fails with its install hint around no view, and the narration on stderr — and verify with stub-engine tests covering: the metrics switch present for a metrics-capable engine, absent for one with no metrics dialect, the log file holding the engine's output, and the not-found hint
- [x] 4.4 Run the program on the alternate screen with `send` wired, the `sup.Wait()` goroutine delivering the engine-exited message, `q`/Ctrl+C and the inherited `SIGINT`/`SIGTERM` handler routing to the helper's stop, serve exiting with the engine's exit status, and `--api` adding the listener and its shutdown, and verify with the existing foreground-serve test patterns: the stop-on-request success path, the non-zero exit path, and the non-terminal `--api` run answering its API as before
- [x] 4.5 Update `serve`'s long help to describe the view and its keys, and verify `go run ./cmd/spinloop serve --help` shows the new text

## 5. Docs and verification

- [x] 5.1 Update `docs/commands/serve.md` for the view: its layout, its keys, the terminal/non-terminal split, and `serve --api`'s log endpoint serving the captured log, and verify the file reads back consistent with the spec delta
- [x] 5.2 Run `gofmt -l .`, `go vet ./...` and `go test ./... -cover`, and verify all are clean with total coverage at or above 80%
- [x] 5.3 Smoke both paths by hand: on a terminal, run `spinloop serve` against a stub engine and confirm the view opens with gauge and bar per series, the log follows, the arrows scroll and stick to the tail, `f` pauses, and `q` stops the engine cleanly; off the terminal, confirm `spinloop serve | cat` forwards the engine's output exactly as before, and verify by the observed behaviour
