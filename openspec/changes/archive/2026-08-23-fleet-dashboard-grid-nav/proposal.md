## Why

`fleet dashboard`'s up/down keys move the selection by ±1 through the flat,
fleet-file-ordered list of tiles, not by a grid row. That happens to look
right when the dashboard renders one tile per row, but as soon as the
terminal is wide enough to fit two or more tiles per row, `down` just
advances to the next tile in file order — usually the one to the right, not
the one below — and there is no way to move left or right at all
([#130](https://github.com/spinloop-ai/spinloop/issues/130)). The arrow keys
need to reflect the grid the operator is actually looking at.

## What Changes

- `up`/`down` move the selection by a full grid row — to the tile directly
  above or below the current one, keeping the same column — instead of by
  one flat index.
- `left`/`right` are added, moving the selection by one tile within the
  current row.
- **BREAKING**: the `j`/`k` vim-style aliases for `down`/`up` are removed.
  Movement is arrow-keys-only, so the footer hint and behaviour stay in sync
  with one clearly documented set of keys rather than mixing two
  conventions.
- Movement never wraps and never jumps sideways to a different column: up on
  the top row and down on the last row are no-ops, not a clamp to an
  absolute index. The one exception is a short last row (fewer tiles than
  the column count) — down onto it lands on whichever tile the row actually
  has, since that row genuinely exists, just narrower than the grid.
- Grid geometry (columns per row) is computed the same way selection math and
  rendering already agree on it (`dashCols`), so a resize that changes the
  column count keeps selection movement consistent with what is drawn.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `fleet-client`: the dashboard's selection movement changes from "moves in
  the fleet file's order" to grid-aware movement — up/down by row, left/right
  by column, both clamped at the grid's edges.

## Impact

- **Go**: `cmd/spinloop/dashboard_model.go` (the key switch's up/down handling
  and new left/right cases, cursor math against `dashCols`); possibly
  `cmd/spinloop/dashboard_render.go` if row/column helpers need to be shared
  rather than recomputed; `cmd/spinloop/fleet_dashboard_test.go` (new tests for
  row-aware up/down, new left/right tests, edge-clamping on a short last
  row, and coverage for a terminal resize changing the column count mid
  session).
