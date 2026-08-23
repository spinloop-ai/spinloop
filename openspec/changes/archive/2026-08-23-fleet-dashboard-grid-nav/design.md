## Context

`dashModel` (`cmd/outfit/dashboard_model.go`) holds selection as a single flat
`cursor int` over `entries []dashEntry`, in fleet-file order. There is no
stored grid — the column count is derived at render and scroll time from
`dashCols(w int) int` (`cmd/outfit/dashboard_render.go`), which fits as many
tiles per row as the current terminal width allows, and `dashGridRows` chunks
the flat tile slice into rows of that width for drawing. `up`/`down` (aliased
to `k`/`j`) currently move `cursor` by ±1; there is no `left`/`right` case.
Because the column count can change on every resize, "grid position" is not a
fixed property of a tile — it is `cursor`'s row/column under whatever
`dashCols` returns right now.

## Goals / Non-Goals

**Goals:**
- Up/down and left/right move the selection the way they look like they
  should, against the grid currently on screen.
- Selection math has one source of truth for the column count (`dashCols`),
  matching what rendering and scrolling already use — no second column
  computation that could drift from the drawn grid.

**Non-Goals:**
- Wraparound in any direction (left off the first column to the last, or
  bottom row back to the top) — not requested by the issue, and the existing
  up/down clamp at the ends is being kept as the model for all four
  directions.
- Any change to how tiles are laid out, spaced, or chosen for a row —
  `dashCols`/`dashGridRows` stay as they are; this only changes how `cursor`
  moves against their output.
- Vim-style scrolling of the whole grid, or a mouse-driven selection — out of
  scope for this fix.

## Decisions

**Up/down move by `dashCols(effWidth())`, left/right by ±1; a move with
nothing to land on is a no-op, never a clamp to an absolute index.**
`cursor`'s row is `cursor / cols` and column is `cursor % cols` for whatever
`cols := dashCols(m.effWidth())` returns at the moment the key is pressed —
recomputed on every keypress rather than cached on the model, so a resize
between two keypresses is picked up for free without an explicit
invalidation step.

Up: target `cursor - cols`; move there only if it's `>= 0`, otherwise stay
put. Every row above the last is fully populated (only the last row can be
short), so whenever a row above exists at all, it has a tile in the current
column — there is no short-first-row case to handle, and clamping to `0`
would be wrong: it would drag a non-leftmost column back to column 0 instead
of leaving the selection where it was.

Down is the case that does need to distinguish two reasons the direct target
can be invalid. Target `cursor + cols`; if that index exists, move there. If
not, compare the cursor's row (`cursor / cols`) to the grid's last row
(`(len(entries)-1) / cols`): if the cursor is already on the last row, there
is no row below at all, so stay put — not a jump to `len(entries)-1`, which
would drag an early column of the last row over to its rightmost tile. If
the cursor is on the row just above a genuinely shorter last row, land on
`len(entries)-1`, the last tile that row has — this is the short-last-row
case, and it's the only situation where the target column doesn't exist but
a different tile in the same row does.

Right: target `cursor + 1`, but only within the current row (`col+1 < cols`
and the target index exists) — otherwise no move; there is no row to fall
back into, so a plain no-op is already correct with no separate case needed.
Left: target `cursor - 1`, only if `col > 0` — otherwise no move, for the
same reason.

Alternative considered: give `dashEntry` a stored `{row, col}` and recompute
it whenever the grid reflows (on resize and on fleet-list changes). Rejected
— it duplicates `dashGridRows`' own chunking logic as a second computation
that has to be kept in sync with it, for no benefit over doing the same
division/modulo arithmetic at keypress time, which is cheap and already the
pattern `dashClampScroll` uses for its own row math.

**Right/left do not cross row boundaries.** Reaching the last tile in a row
and pressing right does nothing, rather than continuing on to the first tile
of the next row (and symmetrically for left at the start of a row). This
matches a physical grid — "right" means "the tile to my right", which does
not exist past the row's last column — and keeps left/right and up/down
orthogonal: only up/down ever change row, only left/right ever change column.

**Movement is arrow-keys-only; the `j`/`k` aliases are dropped.** No
vim-style aliases are added for `left`/`right`, and the existing `j`/`k`
aliases for `down`/`up` are removed rather than kept alongside them — one
set of keys to document and show in the footer, instead of two overlapping
conventions.

## Risks / Trade-offs

- [Recomputing `dashCols` on every keypress instead of caching it] →
  cheap (an integer division over a width already tracked on the model), and
  avoids a stale-cache bug on resize; not a real cost.
- [A short last row makes `down`'s only valid move land on a tile in a
  different column than the one the operator started in] → unavoidable once
  the row has nothing at that column, and only happens when a row genuinely
  exists below (distinguished from "no row below" by comparing the cursor's
  row to the grid's last row) — the operator can see the row they landed in
  is short and why. This is deliberately not treated as "clamp to whatever
  the last valid index is": that reading previously made both up (on the
  top row) and down (on the last row) able to jump sideways to a different
  column whenever the naive target index was merely out of range, rather
  than only when a shorter row was genuinely the target — reported against
  the first cut of this change and fixed before it shipped. Up never faces
  the equivalent case at all, because only the last row can be short.
