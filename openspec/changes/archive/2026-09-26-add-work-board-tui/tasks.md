# Tasks: add-work-board-tui

## 1. Command surface and gate

- [x] 1.1 Create `cmd/spinloop/work_board.go`: `workBoardCmd()` with `workAPIFlags`, `Args: cobra.NoArgs`, a lowercase imperative short/long help in the family's voice; register it on `workCmd()` in `work.go` beside its siblings.
- [x] 1.2 Gate on `term.IsTerminal(os.Stdout.Fd())` before anything else: refuse with an error naming `spinloop work list` as the pipe's command, tested through `cmdWork` under `captureStdout`.
- [x] 1.3 Start the program: `tea.NewProgram(&m, tea.WithAltScreen())` — no `prog.Send` handle on the model, background work returns through the `tea.Cmd`s `Update` yields; wire `--url`/token through `workTarget` so a missing `--url` fails naming the flag before any call.

## 2. Model: reads and ticks

- [x] 2.1 Build `work_board_model.go`: the model struct (items, cursor, detail state, confirm state, status line, busy flag, width/height, `send`), `workBoardRefreshInterval`/`workBoardTailInterval`/`workBoardStaleAfter`/`workBoardNow` as package vars; `Init` kicking the first round.
- [x] 2.2 Poll `GET /v1/items` in a self-rescheduling tick `tea.Cmd` through `workRequest`, one round in flight; decode into `[]orchestrator.ItemView`; discard a reply older than the reading on screen.
- [x] 2.3 Failed round: keep the last reading, mark the title bar with its age once past `workBoardStaleAfter`, dimmed; recover silently on the next good read. `r` restarts a round at once.

## 3. Rendering: columns and cards

- [x] 3.1 `work_board_render.go`: four fixed columns (Backlog, Running, Done, Failed) with counted headers, widths `(W − gaps) / 4` with floors, content pre-sized to exactly W×H, lines clipped with `ansi.CutWc`, title bar shared in shape with the dashboard's.
- [x] 3.2 Cards: id, clipped instructions, priority where non-zero; running adds node and elapsed-time-counts-up from `workBoardNow`; done/failed draw their state colour from `workListColouredState`'s same values; selected card framed in `brandAccent` alone.
- [x] 3.3 Column windowing: show the selection's neighbourhood when a column overflows, clamped; re-clamp the cursor after every read.

## 4. Selection, detail and tail

- [x] 4.1 Arrow-key cursor as (column, row): up/down within a column, left/right to the nearest column holding a card; `enter` opens detail, `esc` returns; no quit from the detail.
- [x] 4.2 Detail pane from the latest `ItemView`: full instructions, dir, tags, timings, failure reason; empty log pane is empty, not a fault.
- [x] 4.3 Tail via `workLogFetch` on `workBoardTailInterval`, appending only the suffix beyond the last fetch; stop on the item ending or a 404; generation counter discards replies after close/reopen.

## 5. Actions and status line

- [x] 5.1 `beginAction`-style in-flight slot: `a` aborts a running card, `x` removes a selected non-running card, each calling the API's path through `workRequest` in a `tea.Cmd` goroutine batched with the spinner chain (`tea.Batch`, fed back through the queue — not `prog.Send` from `Update`).
- [x] 5.2 Status line underway while in flight with `spinnerFrame` on a repaint chain; answer (or `workAPIErr` refusal, verbatim; or the 30s bound's fault) lands on the status line and the board keeps drawing.
- [x] 5.3 Removal confirm in the footer: `y` sends, `n`/`esc`/anything else declines, defaulting to no; a declined question sends nothing and says so.

## 6. The add form

- [x] 6.1 Add `charmbracelet/bubbles` to `go.mod`; import only its `textinput` — nothing else in the board draws from it.
- [x] 6.2 Mode stack: keys route form → removal confirm → detail → board; `n` opens the form; reads keep their cadence and the board repaints behind an open form, the selection untouched until it closes.
- [x] 6.3 The flat form: framed over the dimmed board, five labelled fields with the required three (id, instructions, dir) marked, accent field cursor; each field a `bubbles/textinput` with blink off and `Focus` following the cursor; `up`/`down` step the cursor, caret keys (left/right, home/end) belong to the focused field, `enter`/`tab` advance and the last sends; height clamped to the screen.
- [x] 6.4 The priority textinput filters its input to digits and a leading minus, rejecting the rest without an API call; nothing else is validated client-side.
- [x] 6.5 Send rides the action slot: `POST /v1/items` through `workRequest` with the `workAddBody` shape, spinner while in flight; acceptance closes the form, says added on the status line, and kicks a refresh round at once.
- [x] 6.6 A refusal renders inside the form under the fields, verbatim from the `workAPIErr`, the form held open for a corrected send.
- [x] 6.7 Escape: an empty form closes saying nothing was added; with anything entered, a discard question defaulting to keep — discard closes sending nothing and says so, a kept form stands with its buffers.

## 7. Keys, hints and completion

- [x] 7.1 `q`/`ctrl+c` quit cleanly from the board (terminal restored); interrupt handled.
- [x] 7.2 Footer key hints built from the selection's state and the current mode (board, detail, confirm, form), so a key — `n` included — is named only where it would do something.
- [x] 7.3 Confirm `__complete` behaves for `work board` (no positionals, `--url` etc. offered, nothing on stderr).

## 8. Tests

- [x] 8.1 Fake work list API over `httptest` serving `/v1/items` (GET and POST, the POST accepting and refusing with the API's shapes — a carried id is a 409), `/v1/items/{id}/log` and `/abort`; drive the model directly with injected `tea.KeyMsg`s and shortened interval vars, fixed `workBoardNow`.
- [x] 8.2 Byte-stable render tests: cold run (four empty columns with zero counts), mixed states, oversized column windowing, clipped instructions, aged title bar; cursor/selection marking; the form at a fixed size, with a refusal standing under its fields.
- [x] 8.3 Behaviour tests: card moves on refresh, tail appends and stops at end/404, abort/remove round-trips incl. refusals kept on screen, confirm decline sends nothing, dropped API ages and recovers without exit.
- [x] 8.4 Add-form tests: end-to-end add with the card appearing under Backlog after the kicked read, refusal corrected in-form and resent, escape empty vs typed-with-decline, priority filter rejecting non-numerics keystroke-wise, board keeps moving behind the open form; the form renders byte-stable with blink off.
- [x] 8.5 One `teatest` program-level smoke test at a fixed term size; keep package coverage ≥80% (`go test ./... -cover`).

## 9. Docs and spec

- [x] 9.1 `docs/commands/work.md`: a `spinloop work board` section — keys including the add form, actions, the terminal gate naming `work list`; cross-link from `docs/guides/work-items.md`.
- [x] 9.2 `CHANGELOG.md` entry under unreleased.
- [x] 9.3 `gofmt`, `go vet`, full suite green; `openspec validate add-work-board-tui --strict` clean.
