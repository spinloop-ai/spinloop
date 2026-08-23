## 1. Grid-aware selection

- [x] 1.1 Replace the flat ±1 `up`/`down` cursor moves in `dashModel.Update`
      (`cmd/outfit/dashboard_model.go`) with row moves: `cols :=
      dashCols(m.effWidth())`; down targets `cursor + cols`; up targets
      `cursor - cols`; drop the `j`/`k` aliases so movement is
      arrow-keys-only
- [x] 1.1a Fix up/down so a move with no row to land on is a no-op, not a
      clamp to index `0` or `len(entries)-1` — clamping by absolute index
      drags the selection sideways to a different column, which up on the
      top row and down on the last row must never do. Down still steers
      onto the existing tile of a genuinely shorter row below (the
      short-last-row case), distinguished from "no row below at all" by
      comparing the cursor's row to the grid's last row
- [x] 1.2 Add `left`/`right` cases: move `cursor` by ±1 only within the
      current row (`col := cursor % cols`; right only if `col+1 < cols` and
      the target index exists; left only if `col > 0`), otherwise no move
- [x] 1.3 Call `keepVisible()` after every one of the four directions, as the
      existing up/down cases already do
- [x] 1.4 Update the footer hint and the `fleet dashboard` command's long
      help text to describe arrow-key movement, dropping the `j`/`k` mention

## 2. Tests

- [x] 2.1 Update `TestDashModelSelectionScroll` and
      `TestDashModelUpNavigationClamps` (`cmd/outfit/fleet_dashboard_test.go`)
      for row-based up/down movement in a multi-column layout
- [x] 2.2 Add tests for left/right moving within a row and clamping (no wrap)
      at the first and last column
- [x] 2.3 Add a test for `down` clamping on a short, partially-filled last
      row (fewer tiles than the column count)
- [x] 2.4 Add a test for a resize changing the column count between two
      keypresses, asserting the next move follows the new grid
- [x] 2.5 Extend `TestDashModelEmptyFleet` to also send `left`/`right`
      against zero entries, and drop `j`/`k` from its key list
- [x] 2.6 Add regression tests for 1.1a: up on a non-leftmost column of the
      top row stays put (does not jump to column 0), and down on a
      non-leftmost column of the last row stays put (does not jump to the
      last entry)
- [x] 2.7 Add tests for right's own short-last-row case (a column that fits
      the grid's width but has no tile in that row) and for left/right being
      a no-op in a single-column layout

## 3. Verification

- [x] 3.1 `go vet ./...`, `go test ./...` (coverage stays >= 80%), `gofmt`
- [x] 3.2 Manually run `outfit fleet dashboard` against a fleet file with
      enough nodes to wrap to a second row, and confirm all four arrow keys
      move the selection the way it looks like they should, at more than one
      terminal width, and that `j`/`k` no longer do anything
