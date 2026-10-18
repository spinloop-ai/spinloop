# Design

## Context

`sparklineBlock` in `cmd/spinloop/metrics_render.go` is the single point
where the bar format's colouring happens. It is shared by
`cloud metrics --format=bar`, `fleet metrics --format=bar`, the
dashboard's bar-format tiles, and the serve view's combined
gauge/sparkline rendering, so one change covers every surface. Today it
pools the retained samples to the line width with `poolMax` — keeping each
pool's highest reading, because a sparkline is about pressure and the
thresholds are about peaks — draws one block glyph per column, and applies
the state colour to the final glyph only, on the gauge's 80/90
thresholds; every other glyph is plain.

## Goals / Non-Goals

**Goals:**

- Every pooled column takes the colour of its own value: red above 90%,
  yellow from 80% to 90%, the terminal's default colour below 80%.
- The latest point is treated like any other point.
- The gauge's green/yellow/red scheme and the no-history fallback's
  gauge-styled drawing are untouched.

**Non-Goals:**

- Changing the thresholds — 80/90 stay, as the gauge's.
- Colouring the no-history fallback, which is drawn in the gauge's filled
  style and keeps the gauge's scheme.
- A new state colour: the sparkline uses the two warning colours and
  nothing else.

## Decisions

- **Per-point colouring on the pooled columns, not the raw samples.** The
  line draws one glyph per column after `poolMax`, so the colour is applied
  to the pooled value the glyph stands for. A pool keeps its peak, so a red
  spike that pooled into a column keeps its red whether or not it was that
  column's latest sample.
- **Green is dropped from the sparkline.** The sparkline reads as "the
  crossings are what to read": anything that has not crossed the warning
  threshold is the terminal's default colour. Colouring every below-80%
  point green would paint the calm window and bury the yellow and red it
  exists to show. The gauge keeps green, because it exists to say whether
  the current reading is healthy.
- **No new palette entries.** The state colours already exist
  (`ansiYellow`, `ansiRed`); a below-threshold point simply writes its
  glyph without ANSI decoration.

## Risks / Trade-offs

- A sustained yellow or red window is busier than today's — that is the
  point: it is the shape of the pressure to see.
- Colour-blind terminals already render the two warning colours via the
  terminal's own palette; nothing here changes that.
- The `poolMax` comment already explains why peaks are kept; the colouring
  change does not alter the pooling, so the rationale holds unchanged.
