# Tasks

## 1. Spec delta

- [x] 1.1 Rewrite the "Colour thresholds" requirement in the change's `remote-metrics-bar-format` spec delta: every point colour-coded by its own value — yellow from 80% to 90%, red above 90%, the terminal's default colour below 80% — with scenarios for a historical red point, a historical yellow point, the latest point coloured by its own value, and a fully below-threshold window. The main spec is updated when the change is archived.
- [x] 1.2 Update the bar-format sentence of the "The metrics report's tabular display" requirement in the change's `remote-stats` spec delta, from the "latest point colour-coded" wording to the per-point wording.

## 2. Rendering (cmd/spinloop/metrics_render.go)

- [x] 2.1 Rework `sparklineBlock` to colour each pooled column by its own value — red above 90, yellow from 80, plain below 80, the final column like any other — and update the comments that describe the latest-point-only colouring; verify with the colour tests below and `go test ./cmd/spinloop/`.

## 3. Tests

- [x] 3.1 Rework the colour assertions in `cmd/spinloop/metrics_render_test.go`: `TestRenderSparkline` (a below-threshold latest point is no longer green), `TestRenderSparklineColoursOnlyTheLastPoint` replaced by `TestRenderSparklineColoursEveryPoint` (a red spike and a yellow reading keep their colour where they sit) and `TestRenderSparklineColoursTheLastPointByItsValue` (the final point coloured by its own value, boundary-inclusive at 80 and 90), `TestRenderStatBarsDrawsHistory`, `TestRenderStatCombined`, `TestRenderStatCombinedStoppedEngineDrawsHistoryAlone`, and `TestFormatMetricsBarStoppedWithHistory`.
- [x] 3.2 Update `TestDashModelFormatToggle` in `cmd/spinloop/fleet_dashboard_test.go`: the CPU sparkline's final 20% glyph is no longer green.

## 4. Docs

- [x] 4.1 Update `docs/commands/cloud.md`: the bar-format description no longer says "only the latest point coloured"; it states the per-point colouring on the gauge's thresholds.

## 5. Verify

- [x] 5.1 `go test ./...`, `go vet ./...`, and `gofmt -l .` clean, with total coverage at or above 80%.
