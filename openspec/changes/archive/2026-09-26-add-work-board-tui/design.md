## Context

The fleet dashboard (`cmd/spinloop/dashboard*.go`, `fleet_dashboard.go`) is
the CLI's only full-screen view today, and its architecture is the pattern
this board follows: a small command file that gates on a terminal and starts
the program, a pure model whose intervals and clock are package variables,
and renderers that pre-size content to exactly W×H before lipgloss sees it.
The work family (`cmd/spinloop/work.go`, `work_logs.go`) already holds
every piece of client plumbing the board needs: `workAPIFlags`,
`workTarget`, `workRequest` (whose `workAPIErr` keeps the refusal status),
and `workLogFetch`, whose callers diff the whole kept log against what they
last saw — the log endpoint serves the entire kept output, there is no
server-side offset. `orchestrator.ItemView` is the wire shape, carrying
everything a card and a detail need. No server-side change is involved;
see proposal.md for motivation and the two delta specs for behaviour.

## Goals / Non-Goals

**Goals:**

- A board that reads as the same tool as the fleet dashboard: same
  decoration contract, same accent/state colour discipline, same
  testability (everything but the terminal gate drivable without one).
- The board degrades like the dashboard, not like a crash: a dropped API
  ages the reading, it does not close the view.

**Non-Goals:**

- No reordering by priority, no dragging, no editing an item from the
  board, no user-defined columns or filters, and no `--follow`-style flag:
  the board is always live. Adding, aborting and removing are the whole
  action set; `work add` stays the scripted way in.
- No widgets for the board itself: the grid, cards, detail and hints stay
  hand-rolled under the dashboard's W×H contract — there is no kanban
  component to buy, and bought styling would have to be bent to the
  cli-ux colour discipline. One component is bought, for the form's text
  input alone: `charmbracelet/bubbles`, its `textinput`.

## Decisions

**Three files mirroring the dashboard split.** `work_board.go` builds the
Cobra command (registered on `workCmd()` beside its siblings, `--url` and
token flags via `workAPIFlags`, `Args: cobra.NoArgs`), checks
`term.IsTerminal(os.Stdout.Fd())` before anything else — refusing and
naming `spinloop work list`, the way `runFleetDashboard` names
`fleet metrics --watch` — and runs `tea.NewProgram(&m, tea.WithAltScreen())`. Background work
reaches the loop only through the `tea.Cmd`s `Update` returns — the model
carries no `prog.Send` handle, because calling `Send` from inside
`Update` deadlocks the event loop (the loop reads its own channel on the
`Update` goroutine). `work_board_model.go` holds the model, ticks and keys;
`work_board_render.go` the drawing. Alternative: one file — rejected for
the same reason the dashboard's three exist: the renderers are byte-tested
in isolation.

**One poller, not two, and a race guard.** A self-rescheduling `tea.Tick`
(`workBoardRefreshInterval`, 5s, a package var so tests shorten it) calls
`GET /v1/items` through `workRequest` in a `tea.Cmd` goroutine, one round
in flight at a time. Replies carrying an older stamp than what is on the
board are discarded — the dashboard's guard, copied. A failed read keeps
the last items and marks the title bar with the reading's age once it is
older than `workBoardStaleAfter` (3× the cadence), dimmed, never presented
as current: the cli-ux aged-information rule, met at the title bar rather
than per column. `r` restarts a round immediately.

**Columns are laid out arithmetically, not by a table widget.** The body
splits its width four ways (`(W − gaps) / 4` per column, floors at 1);
each column is a rounded frame whose header names it and counts its cards;
cards inside are fixed-height blocks of pre-clipped lines (`ansi.CutWc`,
never wrap), the way dashboard tiles are joined line-by-line to stay
rectangular. A column taller than its cards shows a window around the
selection, clamped like `dashClampScroll`. Card order is the API's list
order — file order — matching `work list`; sorting backlog by priority was
considered and rejected so two surfaces do not order the same list
differently, and the priority rides on the card instead. Running cards
draw elapsed time from an injected `workBoardNow` var, so counts-up is
testable and never renders stale.

**Selection is a (column, row) cursor; the accent marks it alone.**
Left/right step between columns (into the nearest column holding a card,
skipping empties), up/down move within and scroll the column; the selected
card's frame takes `brandAccent`, its state colour stays on the state
word. After each refresh the row is re-clamped — an item that moved
columns or left the list simply shrinks the column the cursor stands in.

**Detail reuses the board's own data plus the log tail.** Enter shows the
selected `ItemView` from the latest read (there is no single-item GET —
`GET /v1/items/{id}` is a 405 on purpose) and starts a second tick
(`workBoardTailInterval`, 1s, matching `workLogsInterval`) fetching
`workLogFetch` and appending the suffix beyond what it last saw, exactly
`printWorkLogSuffix`'s trick but into the pane; an item ending or leaving
the list stops the tail (404 ends it cleanly, as a follow ends). A
generation counter, copied from `dashboard_detail.go`, discards replies
that arrive after the pane closed or reopened on another item. Escape
returns; the detail pane offers no quit, as the dashboard detail does not.

**Actions ride the model's action slot, not the tick.** `a` and `x` call
`beginAction`, which returns `tea.Batch(workCmd, workBoardSpinCmd())`:
status line "aborting item X…" plus the one spinner (`spinnerFrame`) on a
100ms repaint chain. The spin chain re-arms through the Batch, never
through `prog.Send` from inside `Update` — that deadlocks the loop. This
runs while the call is in flight — abort can block server-side for the stop
grace, and the cli-ux long-operation rule wants motion. `workRequestBound`
(30s) bounds the call; a timeout lands on the status line as a fault and
the board goes on drawing. Refusals come back as `workAPIErr` and are
printed verbatim — the refusal reads the way the API states it, here too.
Removal confirmation is a tiny modal in the footer (y / n / esc, default
no), patterned on the dashboard's keep prompt minus its text input; no
skip-the-question flag, because an unattended run cannot reach a full-screen
view at all — the terminal gate already refuses it, naming `work list`.

**Adding is a flat form modal, not a sub-screen.** `n` opens a framed
form over the dimmed board, showing all five fields at once — id,
instructions, dir, tags, priority — the three required ones marked, an
accent cursor standing on the field that takes keystrokes. `up`/`down`
step the cursor between fields, typing edits the field under it, `enter`
advances and sends from the last. Only the active field carries a caret,
its value clipped around the caret the way a card clips its instructions.
Each field is a `bubbles/textinput`: caret typing, UTF-8-safe backspace,
word motions and horizontal clipping around the caret are its own — the
honest line is that hand-writing a caret editor for five fields is false
modesty, while the grid around it has no component to buy. `Focus`
follows the field cursor; blink is off, both because a standing surface
should not animate by itself and so test view bytes stay stable. Caret
keys (left/right, home/end) belong to the focused field; `up`/`down`
step the cursor and `tab` also advances. A sub-screen was the alternative; it buys a full width five
short fields do not need and costs a navigation mode the mode stack
already carries. Keys route through that stack — form, then removal
confirm, then detail, then board — and reads keep their cadence behind an
open form, the board repainting under it. Sending rides the action slot:
`POST /v1/items` through `workRequest`, spinner on the status line while
in flight; acceptance closes the form, says added, and kicks a refresh
round at once. A refusal lands inside the form under the fields, the API
its own sole validator as ever, and the operator steps the cursor to the
offending field and sends again. Priority's textinput filters its input to digits and a leading
minus, because a non-number could not even be asked of the API. Escape
on an empty form closes and says so;
with anything entered it asks once, defaulting to keep, the keep answer
leaving the buffers where they stand.

**Key set.** Board: `q`/`ctrl+c` quit · arrows move · `enter` detail ·
`n` new item · `a` abort (running only) · `x` remove (not running) ·
`r` refresh. Detail: `esc` back. Form: `up`/`down` field cursor,
`enter` advance/send, `esc` guarded discard. Remove-confirm:
`y`/`n`/`esc`. Footer hints are built per selection state and mode so a
key is named only where it would do something.

## Risks / Trade-offs

- **Whole-log fetch per tail tick is O(log size)** → same cost the
  existing follow already pays; kept output is bounded by the orchestrator.
  A server-side offset endpoint would be the fix if logs ever grow past
  that, and is deliberately not built now.
- **The detail pane's item fields age with the board's last read** — an
  item can start while its detail stands open and its node line will lag
  up to one cadence → acceptable for v1; the tail itself is live, and the
  board's aged-reading mark covers a dropped API. Noted, not papered over.
- **Keyboard-only interaction on narrow terminals** → floors of one card
  per column and clipped everything, as the dashboard does; below a sane
  width columns go one-per-row with the cursor's column filling the body.
- **State drift under the cursor** (item removed by another client while
  selected) → actions report the API's 404 as stated and the next read
  re-clamps; nothing is cached as truth client-side.

## Migration Plan

Additive: a new subcommand over an existing API. Roll back by reverting
the change; no data, no API, no config moves.
