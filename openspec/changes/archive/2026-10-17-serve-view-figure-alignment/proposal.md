# Proposal

## Why

The serve view's combined line draws the figure of the reading unpadded,
between its gauge half and its bar half, so the bar's position depends on
the figure's digit count: a one-digit reading shifts the sparkline one
column left, and 100% — three digits — shifts it one column right. The view
is live and refreshes on its own cadence, so a series whose reading crosses
a digit boundary visibly jitters its bar from one frame to the next. The
serve view is the only surface where the figure sits before the bar: the
dashboard's tiles and the bar format draw the figure after a fixed-width
block, and a digit-count change is invisible there.

## What Changes

- The serve view's figure draws in a fixed-width field — three digits,
  right-aligned, plus the percent sign — so a reading of one, two or three
  digits draws the line the same width, with the bar half starting in the
  same column.
- The combined line is one column wider than a two-digit reading drew it:
  the field is sized to the widest figure a percent can take, not to the
  common one.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `local-serving`: the "The serve view" requirement gains the fixed-width
  figure field — the reading's digit count keeps the line the same width
  and the bar half in the same column — with a scenario for a reading that
  crosses a digit boundary.

## Impact

- `cmd/spinloop/metrics_render.go`: `renderStatCombined` only — the format
  verb of the one line it draws, and its doc comment. `renderGauge`,
  `renderSparkline` and the dashboard's tiles are untouched: they already
  draw the figure after a fixed-width block.
- Tests in `cmd/spinloop/metrics_render_test.go`: the combined-format
  expectations gain the field's padding, and a new test asserts the bar's
  column across digit counts.
