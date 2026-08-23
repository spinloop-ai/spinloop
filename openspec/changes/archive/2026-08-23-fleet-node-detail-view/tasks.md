## 1. Model: screen mode and enter/escape

- [x] 1.1 Add the detail-view mode to `dashModel` (a `detailIdx` or
      `mode`/index pair, `-1`/grid meaning not open) and the `Update` branch
      that reads `<enter>` in grid mode (sets it to `m.cursor`, cursor itself
      unchanged) and `<esc>` in detail mode (clears it, returns to the grid)
- [x] 1.2 Replace the key table while in detail mode: `s`/`x`/`a` act on the
      node the detail view is showing (the same node the cursor points at,
      since entering never moves it); navigation, manual refresh
      (`j`/`k`/arrows/`r`/pgup/pgdown) and quit (`q`/Ctrl+C) are grid-level
      keys and are not read in this mode — escape is the only way out
- [x] 1.3 Confirm the existing stop-confirmation flow (`m.confirm`) works
      unchanged from inside detail mode — it already keys off `m.cursor`,
      and its own `q`/Ctrl+C safety net for quitting mid-confirmation is
      untouched, since the detail view's key table never sees the keypress
      while a confirmation is up
- [x] 1.4 Model tests: enter opens the detail view on the selected node,
      escape returns to the grid with the same selection, start/stop/abort
      from detail mode dispatch to the same node as grid mode would, and
      quit (`q`/Ctrl+C) does nothing and leaves the view open — it is a
      grid-only action

## 2. Log tailing for the node in view

- [x] 2.1 Add the per-detail-view log state to the model: accumulated content
      for the node in view, its next offset, and a generation counter (mirrors
      `fastGen`/`slowGen`) so a reply from a closed or superseded detail view
      is discarded
- [x] 2.2 A `tea.Cmd` that runs `fleet.LogsCall` through `FanOutNodes` over the
      single node in view, started on entering detail mode and rescheduled on
      a repeating tick (a package-level interval variable, following
      `fleetLogsInterval`/`dashboardRefreshInterval`, so tests do not wait on
      it); resumes from the held offset, same stale-cursor handling as
      `fleet logs -f` (a truncated log resumes from wherever it now ends)
- [x] 2.3 Trim the accumulated content to what the pane can show on each poll,
      the same bound `lastLines` applies for `fleet logs`, so a long session
      does not grow an unbounded buffer
- [x] 2.4 Stop the log poll when the detail view closes: gate it on the same
      mode/index state the enter/escape handling owns, so there is one
      shutdown path
- [x] 2.5 Model tests: the log pane accumulates polled content, a stale/
      superseded reply is dropped, content is trimmed to the pane bound, and
      the poll does not continue once escape has closed the view (a poll
      started before escape, replying after, is discarded)

## 3. The detail view renderer

- [x] 3.1 The three-section full-screen layout: metrics section (the node's
      `NodeResult` through the same `renderStatBars`/`renderTokenLines`/
      `renderLastActiveIndented` helpers `dashTileContent` uses, unclipped to
      terminal width), log section (the tailed content, most recent last,
      filling the remaining rows), and a footer naming the mode's keys
- [x] 3.2 Degrade gracefully: a node whose last answer is a failure shows its
      outcome and reason in the metrics section instead of stats; a node
      whose log is missing or empty shows the same explanation `fleet logs`
      gives (`fleetLogsNote`), not an empty pane
- [x] 3.3 An action in flight on the node in view replaces the metrics
      section with the verb and the action's latest status line, the same
      wording the tile already uses
- [x] 3.4 Render tests, byte-stable in the repo's renderer idiom: a running
      node with full metrics, a failing node, a node with no log yet, a node
      with an action in flight, and the footer's key hints
- [x] 3.5 `View` dispatches on the model's mode: grid rendering is unchanged
      when detail mode is not active
- [x] 3.6 A follow toggle (`f`) pauses and resumes the log poll: the tick
      chain keeps running for the life of the view and only starts a round
      when following, so pausing/resuming is a flag flip with no teardown or
      restart; the offset is untouched while paused, so resuming fetches
      the backlog written in the meantime rather than dropping it. The
      header names the state (`following`/`paused`) for a node that can
      poll at all, and says nothing for a standing node, which never polls
      regardless of the flag. Model tests: the toggle pauses and resumes
      dispatch, a tick while paused reschedules but starts no round; a
      `teatest` run confirming paused output is actually withheld and
      resuming delivers it
- [x] 3.7 Narrow `abortAction` to a start in flight (`verb == "start"`
      instead of `!= ""`), on the grid and the detail view — both call the
      same shared function, so this is a single-line change. A stop in
      flight is not abortable: abandoning the dashboard's wait on it would
      leave the operator unsure whether the stop still went through. Model
      tests: `abortAction` itself refuses on a stop in flight (leaves it
      running, does not mark it aborted, does not call its cancel), and the
      refusal holds through both the grid's `a` key and the detail view's
- [x] 3.8 The footer's key help names the abort key only while a start is
      actually in flight on the node it describes (`canAbort`), dropping it
      (`dashFooterHints`) for an idle node, a running one with nothing in
      flight, or one whose in-flight action is a stop — advertising a key
      that would silently do nothing is worse than not advertising it, since
      unlike `s`/`x` an abort with nothing to abort gives no status line
      explaining the no-op. Tests: `canAbort` over idle/start/stop, the
      filter itself, and the grid's and detail view's footers each showing
      the hint only while a start is in flight

## 4. Verification

- [x] 4.1 `teatest` end-to-end: open the dashboard, enter the detail view on a
      fake node, observe the log pane grow across polls (the interval var
      pinned), start the node from the detail view and watch the in-flight
      status replace the metrics section, escape back to the grid with the
      selection intact, quit from inside the detail view
- [x] 4.2 `gofmt`, `go vet ./...`, and `go test ./... -cover` at or above 80%
- [x] 4.3 `openspec validate fleet-node-detail-view --strict` passes clean
- [x] 4.4 Update `AGENTS.md`'s `fleet_dashboard.go`/`dashboard_model.go`/
      `dashboard_render.go` entry: the detail view, its enter/escape keys, and
      the log-tail polling it adds alongside the existing metrics refresh
