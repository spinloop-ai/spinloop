## Context

`spinloop serve` runs its engine in the foreground. Today it forwards the
engine's stdio to the terminal; with `--api` it additionally builds a
`daemon.Daemon` in-process that serves the control API, runs the activity and
history sampler, and starts the engine through a supervisor whose `LogPath`
is empty — so the engine's output still goes to the terminal and the API's
log endpoint answers "missing".

The fleet dashboard's node detail view (`dashboard_detail.go`) is the
reference screen: three stacked sections (metrics, tailed engine log, key
help), the metrics drawn from the same lines `dashNodeView` produces for a
tile, the log polled by byte offset on a one-shot rescheduled tick with a
generation guard, and a footer naming the keys. It reads its node over the
fleet's HTTP seam; serve's node is its own in-process daemon.

`daemon.Daemon` already exposes what the view needs without HTTP:
`Status()`, `Metrics(ctx)`, and the log is a plain file read through
`daemon.ReadLog(path, offset, limit)`. The sampler retains a 10-minute,
40-sample history for the bars. The shared metrics renderers
(`metrics_render.go`) already know both formats: `renderStatBars` draws a
sparkline per series from the history, falling back to the gauge per series
where the history holds none; `renderStatGauges` draws the current reading.
`renderGauge` hard-codes its 25-column width; `renderSparkline` takes one.

bubbletea and lipgloss are existing dependencies; `term.IsTerminal` on
stdout is the terminal gate `fleet dashboard` already uses. See proposal.md
for the motivation; the spec delta in this change is the behaviour contract.

## Goals / Non-Goals

**Goals:**

- The serve view on a terminal: the detail screen's three-section frame, each
  resource series drawn in both formats at once, the engine's log tailed with
  arrow-key scrolling, and the keys and exit the spec delta names.
- One data path: the view reads the in-process daemon the serve process
  already builds for `--api`, so the view works with or without the flag and
  no second log mechanism is introduced.
- A non-terminal serve — with or without `--api` — behaves exactly as today.

**Non-Goals:**

- No changes to the fleet dashboard (it keeps one format at a time with the
  `g` toggle) or to the one-shot `remote metrics` / `fleet metrics` formats.
- No starting, stopping, keeping or aborting from the view; the engine is
  serve's own, and leaving stops it.
- No change to the control API surface: the log endpoint's contract is
  unchanged; a view run merely gives it a file to serve.
- No log capture for non-terminal runs (their `serve --api` log endpoint
  keeps answering "missing", as it does today).

## Decisions

**D1 — The terminal gate is stdout, checked before the engine starts.**
`term.IsTerminal(int(os.Stdout.Fd()))`, the dashboard's own check, decides the
view path after the command is built and printed and `--dry-run` has had its
say. A non-terminal run takes the existing exec path untouched.
*Alternative:* also require stdin to be a terminal — rejected; it adds an
edge the dashboard does not have, and bubbletea's own failure mode covers a
hostile terminal.

**D2 — The view reads the in-process daemon, not its own HTTP API.**
Whenever the view opens — `--api` or not — the serve process runs the engine
through the shared supervised-foreground construction (D8), and the model
calls `d.Status()`, `d.Metrics(ctx)` and `daemon.ReadLog` directly on the
daemon it returns. These are the same functions the API handlers call, so the
view and the API cannot report different facts about the same engine.
*Alternative:* always listen and have the view talk to its own API through
`fleet.daemonNode` — rejected; it adds a socket, a token rule, and a loopback
dependency to a flag that exists to add precisely that, and the dashboard
seam buys serve nothing the in-process calls do not give it.

**D3 — Engine output is captured to the daemon's state-dir engine log.**
The view run gives the supervisor
`filepath.Join(stateDir, "engine.log")` — the daemon's own file — so the
capture holds the engine's whole output from its first line, the view tails
it by byte offset, the status reports its path, and `serve --api`'s log
endpoint serves it. The file outlives the run, as the daemon's does.
*Alternative:* an in-memory ring buffer as the only log — rejected; it loses
the `/v1/logs` improvement and stands up a second log mechanism beside the
daemon's.

**D4 — Both formats are one renderer, per series.**
A new shared function in `metrics_render.go` (the bar and gauge formats'
`barSeriesList` supplies the series and their order: CPU, RAM, each GPU's
utilisation and memory) draws, per series: the gauge of the current reading,
then — where the history holds samples — the sparkline beneath it, the bar
format's no-history rule making a history-less series gauge-only. The gauge
line carries the label; the sparkline line passes an empty label so the two
stack in the label column. `renderGauge` gains a width parameter (its
callers pass today's 25; the view passes `barLineW`), so gauge and bar share
one draw width and line up.
*Alternative:* a gauge block over a bar block — rejected; it separates
"now" from "trend" per resource, which is what the issue's "show both" does
not ask for.

**D5 — The log pane scrolls an in-memory tail, clamped to a line budget.**
The model keeps the tailed content (most recent last) and `behind` — how
many lines the window's bottom sits behind the newest; 0 is at the tail.
Each poll (the detail view's 3-second cadence, same one-shot rescheduled
tick and generation guard) reads from the stored byte offset, appends, and
trims to a line budget (a few thousand lines, well past any pane, bounded in
memory; the file on disk keeps the whole record). `up`/`down` move `behind`
by one, `pgup`/`pgdown` by the pane's height, clamped so the window never
shows past the oldest retained line; while `behind` is 0 new lines keep it
at the tail, and while it is not, the window stays put as new lines arrive.
The displayed slice is the last `paneRows` lines ending `behind` from the
end, so the clamp and the stick are arithmetic on two integers, testable
without a terminal. A `StaleOffset` reply (the file shrank) resets the
offset to the reply's `NextOffset`, the rule the log read defines.
*Alternative:* scroll the file by offset — rejected; `ReadLog` is
forward-only with a 256 KiB read bound, so reaching far back means paging
and mid-line bookkeeping, and the detail view already establishes the
in-memory content pattern the budget generalises.

**D6 — One new file, the frame helpers reused.**
`cmd/spinloop/serve_view.go` holds the model and its `View()`, drawn from
the detail screen's own parts: `dashTitleBar` (screen "serve", the
Spinloop's path and the log's following/paused state to the right), the
three-section frame with dividers, the metrics section's lines
(`dashStateLine`, `dashTileServingLine`, `renderActiveIndented` with no keep
deadline, D4's renderer, `renderTokenLines`), and the footer through the
existing key-hint helpers. The key help names exactly what the view answers
to — `↑↓` scroll, `pgup`/`pgdown` page, `f` follow, `q` quit — and nothing
else, per the cli-ux rule. Stale readings carry their age through the
detail view's own age rule, so a tick that could not renew a reading is not
drawn as current.

**D7 — Refresh cadence matches the local node.**
One 2-second tick (the dashboard's local cadence) takes an in-process
status-plus-metrics reading and replaces the view's; the log poll chain runs
on its own 3-second cadence, independently, exactly as the detail view keeps
its log poll separate from the grid's refresh. A failed reading leaves the
last one in place with its age.

**D8 — One shared construction for the supervised foreground engine.**
The daemon-plus-supervisor plumbing `runServeForegroundAPI` already has —
the supervisor, the daemon with its served name, scrape target, engine
endpoint and refusing start stub, the sampler for the run's life,
`MarkActive`, the signal relay, the graceful stop, and the exit-status rule
(a stop on request is success) — is extracted into one helper that the view
path and the `--api` path both call. It is parameterised only by the
supervisor's log target — the state-dir engine log for a view run, empty for
a non-terminal `--api` run, which forwards stdio as today — and whether the
control API listener comes up, which is `--api`'s own say. Nothing else
differs, so the two paths cannot drift: served, scraped and probed the same
way, stopped the same way, exited the same way. `withMetricsArgs`,
`scrapeTargetFor` and `engineEndpointFor` live inside it (an engine with no
metrics dialect gets no added switch and the host's series). The engine
still starts before the view opens, so a missing binary fails with its
install hint around no view; the view's own pieces — the program on the
alternate screen, `m.send = prog.Send` (the dashboard's own door), the
engine-exited message from the `sup.Wait()` goroutine that the model quits
on, and `q`/Ctrl+C and the inherited `SIGINT`/`SIGTERM` handler routing to
the same stop — sit on top of the helper rather than beside it, and the
narration (the command block and the preset/model line) goes to stderr in
the view path because stdout is the view's screen, passed through
`buildServeArgv` rather than printed there.
*Alternative:* the view path assembles its own daemon setup, kept matched to
`--api`'s by review — rejected; two setup blocks for one object drift, and
the second is where the first's later changes stop being applied. Opening
the view before the engine starts, so a key starts it — also rejected;
serve exists to run the engine, and the engine-not-installed scenario must
keep failing the way it does today.

## Risks / Trade-offs

- [An interactive user who expected raw logs gets a screen] → the
  motivation for the issue; the command and narration still appear (on
  stderr, in the scrollback after exit), piped and scripted runs are
  byte-for-byte unchanged, and `q` leaves as easily as it entered.
- [Ctrl+C on a terminal changes from a hard SIGINT to the view's quit] →
  the quit is a graceful stop with escalation, the behaviour `serve --api`
  already has for Ctrl+C; a hard kill remains available as it always was.
- [The in-memory log tail is a second copy of the log] → bounded by the line
  budget and trimmed to it; the file on disk is the record, and the copy is
  the same pattern the dashboard's detail view already uses.
- [A terminal that is a TTY but cannot do an alternate screen] → the
  dashboard has the same exposure; bubbletea restores the terminal on exit
  whatever key got there, and the non-terminal path is untouched.
- [Metrics for a view run depend on the engine's endpoint being reachable
  from the host] → the scrape target resolution already handles bind,
  BASEURL and the engine defaults in that order, and an engine with no
  metrics endpoint degrades to the host's series rather than failing the
  view.

## Migration Plan

One CLI release; no data or config migration. Non-terminal behaviour is
unchanged, so nothing scripted depends on the new path. Rollback is a revert.

## Open Questions

None that would change the spec or the task breakdown; the log line budget
and exact pane proportions are tunable during implementation.
