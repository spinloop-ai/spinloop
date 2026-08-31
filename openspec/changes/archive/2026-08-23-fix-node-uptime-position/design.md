## Context

`dashTileContent` (`cmd/spinloop/dashboard_render.go`) builds a node tile as
plain text lines, which `dashClip` then hard-truncates to `dashTileW` (42
columns) each. `dashTileServingLine` currently builds `runner  modelID  (up
...)` as one string; when `runner + modelID` alone is long, `dashClip` cuts
the uptime suffix off entirely. See proposal.md - Why.

The tile has two shapes that print a node's state:
- Settled: `dashTileContent`'s `default` case prints `name  state` as the
  tile's first line, then `dashTileReportBody` (serving line, last-active,
  bars) beneath it.
- Action in flight (`a.verb != ""`): the first line is `name  <verb
  progress>`; if the round answered, `state` prints alone as its own line
  before the same `dashTileReportBody`.

## Goals / Non-Goals

**Goals:**
- Uptime is always visible for a running node regardless of runner/model
  name length, within the existing 42-column tile width.
- Keep both tile shapes (settled and in-flight-with-report) visually
  consistent: uptime sits beside whichever line carries the state.

**Non-Goals:**
- Not changing tile width, wrapping behavior, or what facts a tile shows —
  only where uptime renders.
- Not touching last-active, which is a separate figure with its own line
  (`renderLastActiveIndented`) and its own truncation rules already.

## Decisions

- **Append uptime to the state line, not a new line.** Adding a dedicated
  uptime line would push later content (last-active, bars, tokens) down and
  risk overflowing `dashTileH` (12 lines), whose budget is already fully
  used in `TestDashTileRunningByteStable`. Appending `"  (up "+duration+")"`
  after the state keeps line count unchanged.
  - Alternative considered: truncate the model ID instead of the whole
    line so uptime always fits. Rejected — silently shortening the model
    name is a worse loss than reordering, and it still needs conditional
    logic to know how much room uptime needs.
- **`dashTileServingLine` drops the uptime suffix**, keeping just
  `runner  modelID`. The function's doc comment ("the runner and the model,
  then the uptime") gets corrected to describe the new output.
- **A small state-plus-uptime helper** (e.g. `dashStateLine(state string, m
  metrics.Stats) string`) is used in both call sites (the settled
  `default` case's `name  state` line and the in-flight case's bare `state`
  line) so the `(up ...)` formatting isn't duplicated. It only appends
  uptime when `m.UptimeSeconds > 0`, matching the existing guard in
  `dashTileServingLine`.

## Risks / Trade-offs

- [Existing byte-stable tests assert exact tile text and will fail once the
  line layout changes] → Update the fixtures in
  `cmd/spinloop/fleet_dashboard_test.go` (`TestDashTileRunningByteStable`,
  `TestDashTileActionInFlightWithReport`, `TestDashGridRealTilesSideBySide`,
  and any other test asserting a `(up ...)` line) as part of this change.
- [The settled top line (`name  state  (up ...)`) is now longer and could
  itself hit the 42-column clip for a long node name] → Acceptable: node
  names are operator-chosen and typically short; this is no worse than the
  status quo, where the same line already carries `name  state`.
