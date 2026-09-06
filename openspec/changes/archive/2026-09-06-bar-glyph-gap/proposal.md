## Why

The bar format draws each resource series as a sparkline of block glyphs, and a
series at 100% fills the whole cell with the full block (`█`). When several rows
are all high — GPU util and GPU mem pinned, say — their full-height glyphs touch
edge to edge and the rows read as one solid block with no separation between
them, so the eye can't tell where one series ends and the next begins.

## What Changes

- Cap the sparkline's tallest glyph at the seven-eighths block (`▇`) instead of
  the full block (`█`), so the highest value in a series always leaves one-eighth
  of the cell height unfilled above it. Adjacent rows at their maximum now read
  as separate bars with a visible gap between the bottom of one and the top of
  the next.
- The trailing percentage is unchanged and still carries the exact value, so the
  glyph is a visual approximation that may sit one grade below full height.
- The change applies to the sparkline drawing shared by `remote metrics
  --format=bar`, `fleet metrics --format=bar`, and the dashboard tiles. The
  `gauge` format (a horizontal filled/unfilled progress bar) is a different
  drawing and is left as-is.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `remote-metrics-bar-format`: the sparkline's glyph-for-value mapping gains a
  ceiling — the highest value renders as the seven-eighths block (`▇`), never the
  full block (`█`) — so a maxed row leaves a visible gap above it.

## Impact

- Code: `cmd/spinloop/metrics_render.go` — the sparkline's glyph set
  (`barGlyphs`) and the value-to-glyph mapping (`barGlyph`) used by
  `renderSparkline`. No change to `renderGauge`.
- Surfaces: `spinloop remote metrics --format=bar`, `spinloop fleet metrics
  --format=bar`, and the dashboard tiles (all draw the sparkline through
  `renderStatBars`).
- Tests: expectations in `cmd/spinloop/metrics_render_test.go` (and any
  dashboard/fleet assertions on specific sparkline glyphs) that pin a value to a
  full-block glyph or to the old eight-grade bucket boundaries.
- Not breaking in any data or API sense: the change is to a visual approximation;
  the exact value is still reported by the trailing percentage and by the other
  formats.
