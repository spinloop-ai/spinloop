# Tasks

## 1. Spec

- [ ] 1.1 Rewrite the "Colour thresholds" requirement in `openspec/specs/remote-metrics-bar-format/spec.md`: every point colour-coded by its own value — yellow from 80% to 90%, red above 90%, the terminal's default colour below 80% — with scenarios for a historical red point, a historical yellow point, the latest point coloured by its own value, and a fully below-threshold window.
- [ ] 1.2 Update the bar-format sentence of the "The metrics report's tabular display" requirement in `openspec/specs/remote-stats/spec.md` from the "latest point colour-coded" wording to the per-point wording.

## 2. Rendering (cmd/spinloop/metrics_render.go)

- [ ] 2.1 Rework `sparklineBlock` to colour each pooled column by its own value — red above 90, yellow from 80, plain below 80, the final column like any other — and update the comments that describe the latest-point-only colouring.

## 3. Tests

- [ ] 3.1 Rework the colour assertions in `cmd/spinloop/metrics_render_test.go`: `TestRenderSparkline` (a below-threshold latest point is no longer green), `TestRenderSparklineColoursOnlyTheLastPoint` (renamed and rewritten for per-point colouring, including a red spike that is not the latest point), `TestRenderStatBarsDrawsHistory`, `TestRenderStatCombined`, `TestRenderStatCombinedStoppedEngineDrawsHistoryAlone`, and `TestFormatMetricsBarStoppedWithHistory`.
- [ ] 3.2 Update `TestDashModelFormatToggle` in `cmd/spinloop/fleet_dashboard_test.go`: the CPU sparkline's final 20% glyph is no longer green.

## 4. Docs

- [ ] 4.1 Update `docs/commands/cloud.md`: the bar-format description no longer says "only the latest point coloured"; state the per-point colouring on the gauge's thresholds.

## 5. Verify

- [ ] 5.1 `go test ./...`, `go vet ./...`, and `gofmt -l .` clean, with total coverage at or above 80%.
