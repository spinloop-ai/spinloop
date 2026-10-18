# Proposal

## Why

The bar format's sparkline colours only the latest point — green at or
below 80%, yellow to 90%, red above — and leaves every earlier point in the
terminal's default colour (issue #265). A utilisation spike that happened
mid-window and has since passed is invisible in the one drawing that exists
to show pressure: the window is drawn as a line of bars, but a red spike
three minutes back reads as an unremarkable default-colour glyph, so the
user sees a calm line that just happened to be at 95%.

## What Changes

- The sparkline colours every point by its own value, on the gauge's
  80/90 thresholds: yellow for values from 80% to 90%, red for values above
  90%, and the terminal's default colour for values below 80%. Green is
  dropped from the sparkline entirely — the gauge keeps its
  green/yellow/red scheme, since its purpose is the current reading.
- The latest point is no longer special: it takes the colour of its own
  value, like any other point.
- A threshold crossing is readable wherever it sits in the window: a spike
  that passed earlier keeps its colour where it happened, and a calm window
  stays uncoloured.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `remote-metrics-bar-format`: the "Colour thresholds" requirement is
  removed and re-added as "Per-point colour thresholds" — every point of
  the sparkline is colour-coded by its own utilisation, not only the
  latest, and the scenarios cover historical points as well as the final
  one. The removal is forced by the validator, which would not let a
  modified block drop the "Green bar for low utilization" scenario, and
  that scenario can no longer hold: the sparkline no longer draws green.
- `remote-stats`: the one sentence of the "The metrics report's tabular
  display" requirement that describes the bar format's colouring (the
  "latest point") is updated to the per-point wording.

## Impact

- `cmd/spinloop/metrics_render.go`: `sparklineBlock` only — the single
  colouring point shared by `cloud metrics --format=bar`,
  `fleet metrics --format=bar`, the dashboard's bar-format tiles, and the
  serve view's bar half. The gauge and the no-history fallback are
  untouched.
- Tests in `cmd/spinloop/metrics_render_test.go` and
  `cmd/spinloop/fleet_dashboard_test.go` that assert the old
  latest-point-only colouring.
- `docs/commands/cloud.md`: the paragraph describing the bar format's
  "only the latest point coloured".
