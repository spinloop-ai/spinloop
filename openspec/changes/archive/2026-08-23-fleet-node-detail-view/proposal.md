## Why

The dashboard's tile is deliberately small — 42x12, sized to fit a grid of nodes
on screen at once — so it has no room for a node's log output, and its metrics
are clipped to what fits the bar-format block. There is no way to look closely
at one node without leaving the dashboard for `fleet logs <node>` in another
terminal. `<enter>` on the selected node should open a full-screen view of that
one node — metrics, logs, and the keys that work there — and `<esc>` should
return to the grid (lucinate-ai/outfit#125).

## What Changes

- `<enter>` on the selected tile opens a full-screen detail view of that node,
  replacing the grid. `<esc>` returns to the grid with the same node selected.
- The detail view is three stacked sections: the node's metrics (the same
  facts and wording the tile and `fleet metrics` render, unclipped), the
  node's engine log (tailed and followed, in the same wording `fleet logs -f`
  prints for one node), and a footer line of the keys the view answers to.
- The node's live refresh continues in the detail view on its existing per-kind
  cadence (fast tick for daemon nodes, the slower deadline for `kind: remote`),
  and the log pane polls and appends the same way `fleet logs -f` follows one
  node, so both panes update while the view is open.
- Start, stop and abort on the node under view work the same as on the grid:
  the same confirmation for stop, the same in-flight status on the metrics
  pane in place of the tile, so leaving the view mid-action loses nothing.
  `q`/Ctrl+C are grid-level only and do nothing from inside the detail view —
  escape is the only way out of it, so a stray quit keystroke while
  inspecting a node cannot end the session out from under the operator.
- The log's follow can be paused and resumed from the keyboard (`f`),
  independently of the metrics section's own refresh: pausing stops picking
  up new output, and resuming fetches whatever the engine wrote in the
  meantime rather than losing it.
- **BREAKING (behaviour)**: the abort key now drives nothing on a stop in
  flight, on the grid as well as in the detail view — only a start, the one
  action with no deadline of its own, is abortable. Abandoning the wait on a
  stop would leave the operator unsure whether it still went through, so it
  is no longer offered.
- The grid's own layout, refresh, selection and exit are otherwise unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `fleet-client`: the dashboard gains a full-screen per-node detail view
  reachable from the grid with `<enter>`/`<esc>`, showing that node's
  unclipped metrics and followed engine log, with the same start/stop
  behaviour as the grid. The existing "dashboard drives the selected node"
  requirement is also narrowed: abort now applies to a start in flight only,
  on both the grid and the detail view — a stop in flight is not abortable.

## Impact

- `cmd/outfit`: the dashboard model (`dashboard_model.go`) gains the detail
  view's state (which node, its log content and cursor) and the `<enter>`/
  `<esc>` transitions; `dashboard_render.go` gains the full-screen renderer.
  No new files are required by the proposal, though the implementation may
  split the detail view into its own file if the existing ones would grow
  past a comfortable size. `abortAction` — the grid's existing abort handler,
  shared by the detail view — is narrowed to refuse a stop in flight.
- `internal/fleet`: unchanged — the detail view reads logs through the
  existing `Node.Logs` / `fleet.LogsCall`, the same call `fleet logs` already
  uses, and metrics through the existing `fleet.MetricsCall`.
- No new dependencies: the view is built from the same Bubble Tea/lipgloss
  stack the dashboard already uses.
- `fleet logs` and `fleet metrics --watch` are unaffected.
