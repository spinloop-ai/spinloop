## Context

The bar format draws each resource series as a sparkline via `renderSparkline`,
which maps each sample to a block glyph with `barGlyph` over the set `barGlyphs`
(`cmd/spinloop/metrics_render.go`). That one function is the shared drawing path
for `remote metrics --format=bar`, `fleet metrics --format=bar`, and the
dashboard tiles (all route through `renderStatBars`). The `gauge` format is a
separate drawing (`renderGauge`) — a horizontal filled/unfilled bar. Today
`barGlyphs` is the eight block elements `▁▂▃▄▅▆▇█`, so a value at 100% fills the
whole cell with `█` and adjacent maxed rows merge into one solid block (see
proposal.md — Why).

## Goals / Non-Goals

**Goals:**
- Cap the sparkline's tallest glyph at the seven-eighths block (`▇`) so a maxed
  row always leaves a visible gap above it.
- Make "never draws a full block" a property of the glyph set, not a runtime
  clamp.

**Non-Goals:**
- Change the `gauge` format or the bar format's no-history gauge-style fallback.
- Change the colour thresholds, the downsampled-extremes pooling, or the
  trailing percentage.

## Decisions

1. Reduce `barGlyphs` to the seven sub-full block elements `▁▂▃▄▅▆▇` and keep the
   existing `int(pct/100 * len(barGlyphs))` mapping.
   - `len(barGlyphs)` is now 7, so 100% maps to index 7, clamped to 6, which is
     `▇`. The set simply contains no `█`, so a full block is unrepresentable
     rather than clamped away.
   - The seven grades split 0–100% into even bands of roughly 14.3%.
   - Alternative considered: keep the eight elements and clamp the index to 6.
     That changes only the top band (87.5–100% becomes `▇`) and leaves every
     other value's glyph identical — a smaller visible diff — but it expresses
     the cap as a clamp and leaves the top band twice as wide as the rest.
     Rejected: the structural invariant and even scale are cleaner, and the
     mid-range shift is invisible because the trailing percentage still carries
     the exact value.

2. Scope the change to the sparkline only.
   - `renderGauge` keeps `█` as its filled character. The gauge is a horizontal
     progress bar where a full fill at 100% is the natural, expected reading;
     capping it would make a 100% gauge look under-filled. The request and its
     example are about the bar-format sparkline, so the gauge is left as-is.

## Risks / Trade-offs

- [Mid-range glyphs shift one grade] (e.g. 50% `▅` → `▄`) under the seven-grade
  split → the trailing percentage carries the exact value and the sparkline is a
  shape/pressure read, not a per-sample meter; the shift is not something a
  reader relies on. Accepted.
- [The no-history fallback in bar format still draws full `█`] at 100%, since it
  reuses the gauge style → that is a transient pre-history state (a fresh daemon
  with no retained samples); if the same gap is wanted there, it is a small
  follow-up. Out of scope for this change.
- [Existing tests pin specific glyphs] to the old eight-grade boundaries →
  updated in the apply phase; they are the regression net, not a constraint on
  the mapping.

## Open Questions

None. The gauge scope is decided (out of scope); capping it would be a separate
change.
