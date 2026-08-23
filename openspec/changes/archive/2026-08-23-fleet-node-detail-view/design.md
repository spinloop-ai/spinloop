## Context

See proposal.md — Why. The pieces already in place that this builds on:

- `dashModel` (`dashboard_model.go`) already holds one `dashEntry`/`NodeResult`/
  `dashAction` per node, refreshed by kind on `refreshRemoteGroup`, and already
  renders a node's in-flight action on its tile via `dashTileContent`. The
  metrics facts a panel needs come from `fleet.MetricsCall`, already run every
  round over every node — the detail view does not need a node-specific
  metrics read, only a wider rendering of the same `NodeResult` already held
  for the selected node.
- `renderStatBars`, `renderTokenLines` and `renderLastActiveIndented`
  (`metrics_render.go`) write to an `io.Writer` and are not clipped to any
  width — `dashTileContent` is the caller that clips them to 42 columns, not a
  property of the helpers themselves. The detail view can call them directly
  and let its pane be as wide as the terminal.
- `fleet logs -f` (`fleet_logs.go`) is the precedent for tailing and following
  one node's log: `fleet.LogsCall(offsets, limit)` reads from an offset map
  (absent entry means `daemon.TailLog`), `followFleetLogsLoop` polls on
  `fleetLogsInterval` and resumes from `NextOffset`, and a stale cursor (the
  log was truncated) is handled by resuming from wherever the log now ends
  rather than erroring. The detail view's log pane is this same shape run
  against one node, driven by the tea program's own tick instead of a bare
  loop.
- `fleet.LogsCall` is a `Call` (`func(context.Context, Node) NodeResult`), so
  it runs through the same `FanOutNodes` the metrics rounds already use, over
  a one-element node slice for the node in view.

## Goals / Non-Goals

**Goals:**

- Reuse the grid's refresh, action and render machinery for the metrics
  section rather than re-deriving it: the detail view's metrics pane and the
  tile must never be able to disagree, because they draw from the same
  `NodeResult` through the same helpers.
- Add exactly one new kind of polling (the log tail) and one new screen mode,
  both scoped to the dashboard model already in place.
- Leaving and re-entering the detail view must behave exactly as the grid's
  existing exit and navigation paths; quitting is a grid-only action, so the
  detail view has no quit key of its own — escape is the only way out of it.

**Non-Goals:**

- No log filtering, search, or scrollback beyond what fits the pane — the
  view tails the same way `fleet logs -f` does, and does not add controls
  `fleet logs` itself doesn't have.
- No change to `fleet logs`, `fleet metrics --watch`, or the daemon's
  `/v1/logs` contract.
- No detail view for more than one node at a time, and no split/multi-pane
  layout beyond the three fixed sections.

## Decisions

### The model gains a screen mode, not a second `tea.Model`

`dashModel` gains a `mode` field (`dashModeGrid` / `dashModeDetail`, or
equivalently a `detailIdx int` with `-1` meaning "grid") rather than the
detail view being a separate `tea.Model` swapped in by `tea.Program`. The
existing per-node state (`entries`, `results`, `actions`) is addressed by the
same index either way, so `Update` gains one branch per mode instead of two
models needing to share that state through some other channel. `View`
dispatches on the mode: the grid renderer is unchanged, and a new detail
renderer draws the full-screen layout for `entries[detailIdx]`. This mirrors
how the stop confirmation is already handled — a mode flag on the same
model, not a separate program.

Alternative considered: a second `tea.Model` pushed with a Bubble Tea
"stack" pattern. Rejected — it would need the node's `NodeResult`, `dashAction`
and refresh state either copied in on entry (stale the moment a background
refresh lands) or reached back through a pointer to the parent model, which
is the same coupling as a mode flag with an extra layer of indirection.

### Enter/escape, not a new top-level command

`<enter>` sets `detailIdx = m.cursor` (or `mode = dashModeDetail`); `<esc>`
clears it. The cursor itself does not move — leaving the detail view returns
to the same selection. While in detail mode, the key table is replaced: `s`/
`x`/`a` act on `entries[detailIdx]` exactly as they act on `entries[cursor]`
in grid mode (the two are the same node, since entering never changes the
cursor), `f` toggles the log's follow (see below), and `j`/`k`/navigation and
`r` are not read in this mode — the log pane's own scrolling, if any is
needed, is future scope, not this change (see log pane sizing below). `q`/
Ctrl+C are not read either: quitting stays a grid-level action, so the only
way out of the detail view is escape — a deliberate choice, not an oversight,
so a stray quit keystroke while inspecting a node cannot end the whole
session out from under the operator. The stop confirmation overlay already
keyed off `m.confirm` works unchanged in either mode: it reads `m.cursor`,
which is still the node the detail view is showing, and its own `q`/Ctrl+C
handling (an existing grid-level safety net for quitting mid-confirmation) is
untouched by this — the detail view's own key table simply never sees the
keypress while a confirmation is up.

### The log pane is fetched by the same fan-out, on its own tick

A new `tea.Cmd`, started on entering detail mode and on a repeating tick
(reusing the `fleetLogsInterval` pattern as a package variable so tests do
not wait on it), runs `fleet.LogsCall(offsets, limit)` through `FanOutNodes`
over the single node in view and returns the appended content tagged with the
node's name and the detail view's own generation counter (so a reply that
outlives leaving the view, or leaving and re-entering on a different node, is
discarded exactly as stale metrics rounds already are). The model keeps the
per-node log offset and accumulated content only while that node's detail
view is open; closing the view drops it; reopening starts a fresh tail from
`daemon.TailLog`, matching `fleet logs` itself, which does not resume a
previous invocation's position either. The accumulated content is capped to
what the pane can show (trimmed the way `lastLines` trims for `fleet logs`
without `-f`) — the pane does not grow an unbounded buffer over a long
session.

The log poll stops when the detail view closes: it is driven by the same
"is a round in flight for this mode" guard the metrics rounds use, keyed off
`mode`/`detailIdx` rather than a separate boolean, so there is one shutdown
path, not two.

### Layout: three fixed sections, full terminal width

The metrics section renders `entries[detailIdx]`'s `NodeResult` through the
same `renderStatBars`/`renderTokenLines`/`renderLastActiveIndented` helpers
`dashTileContent` calls, unclipped — the terminal width is the only
constraint, versus the tile's fixed 42 columns. The log section shows the
tailed content, most recent lines last, filling the remaining rows between
the metrics section and the footer (a fixed metrics-section height, computed
the same way the tile's 12-line content is sized today, so the log pane's
height is `terminal height - header - metrics height - footer`, floored at a
minimum of a few lines). The footer names the keys live in this mode (`esc
back`, `s start`, `x stop`, `a abort`, `f follow`), replacing the grid's footer
while the detail view is open, following the same pattern the stop-confirm
overlay already uses to replace the footer text.

### Non-goals kept out: no independent log scrollback

`fleet logs -f` never seeks backward past what it printed; the detail view
holds to the same limit rather than growing a pager. If the pane cannot show
the whole tailed buffer, only the most recent lines that fit are shown — the
same trimming `lastLines` already does for the one-shot and follow modes of
`fleet logs`. A full pager is left for a future change if the need shows up
in use; today's ask is "see the log without leaving the dashboard," not "browse
history."

### The follow toggle is a flag the tick reads, not a second chain

`f` flips a `detailLogFollow` bool; the tick chain started on open keeps
ticking for the life of the view regardless of the flag, and only consults it
to decide whether to actually start a round that tick. Pausing therefore
needs no teardown and resuming needs no restart — the next tick simply picks
the flag back up — which keeps the "one shutdown path" property (the chain
still lives and dies with `m.detail`/node-liveness alone) rather than adding
a second lifecycle for follow on top of it. The offset is left untouched
while paused, so resuming is an ordinary poll from where the log already was
— the engine's backlog since pausing is fetched and shown in one go, the same
as any poll that was simply late, rather than the view jumping to "now" and
silently dropping what happened in between.

### Abort is narrowed to a start in flight, on the grid too

`abortAction` (the one function both the grid's `a` and the detail view's `a`
already called) now checks `m.actions[i].verb == "start"` instead of merely
`!= ""`. A stop has no comparable open-ended wait to abandon — it is not a
cold cloud wake with no deadline, it is a call against an engine already
running — and "abandoning" the dashboard's wait on it would leave the
operator unsure whether the stop still went ahead, which is worse than
either waiting for the reply or not offering the key at all. This lands as a
single-line change in the one shared function, so the grid and the detail
view narrow together automatically; nothing in either key table needed
touching; the existing abort scenarios (an in-flight start, an abort that is
not a cancellation) were already written about starts specifically, so only
the requirement's blanket "an action in flight" wording needed correcting to
match what the scenarios always meant.

The key help follows the same narrowing: the static `dashGridKeys`/
`dashDetailKeys` constants stay the single source of truth for wording, and a
small filter (`dashFooterHints`) drops the `"a abort"` entry from whichever
one the frame is about to render when `canAbort()` — a start in flight on the
node under the cursor (grid) or in view (detail) — is false. Advertising a
key that would silently do nothing (an idle node, a running one, or a stop in
flight) is worse than not advertising it: `s`/`x` always attempt something
even when refused (a status line explains why), but abort with nothing to
abort is a pure no-op, and a static hint cannot tell the operator that.
Filtering a string rather than keeping two hand-written constants per screen
keeps the wording itself in one place per screen; only the abort clause's
presence varies.

## Risks / Trade-offs

- [Two live polls per node in view — the existing metrics round and the new
  log tail — could double a slow node's traffic while its detail view is
  open] → Only the node in view gets a log poll; every other node's traffic
  is unchanged. A `kind: remote` node's log read goes through the same signed
  control-plane call its metrics read already does, at the `fleetLogsInterval`
  cadence (seconds), not the slower remote metrics cadence — accepted, since
  it mirrors what `fleet logs -f` already costs against the same node kind.
- [A node's detail view outliving the node itself — closed or reconfigured
  mid-session] → Not reachable: the node set is built once at dashboard
  startup, same as today, and the detail view only ever addresses an index
  into it.
- [Log content held per open detail view could grow if the operator leaves it
  open a long time] → Trimmed to the pane's visible lines on every poll, the
  same bound `fleet logs -f` applies with `--limit`.
- [Coverage floor (80%) over a new surface] → Same test seams as the grid:
  render tests for the detail layout (byte-stable, a running node, a failing
  node, a node with no log yet), model tests for enter/escape and the log-poll
  generation/staleness rules, and a `teatest` run covering entering the detail
  view, watching the log pane grow, and escaping back.

## Open Questions

None blocking. Log pane scrolling/paging is explicitly deferred (see
Decisions); if operators need it in practice it is a follow-up change against
the same model, not a reason to hold this one.
