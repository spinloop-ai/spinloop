## 1. Core change

- [x] 1.1 In `cmd/spinloop/metrics_render.go`, reduce `barGlyphs` from the eight block elements `▁▂▃▄▅▆▇█` to the seven sub-full `▁▂▃▄▅▆▇`, and update its doc comment to say the set stops one grade short of the full block so the tallest row leaves a sliver of space above it. `barGlyph` already keys off `len(barGlyphs)`, so its body is unchanged. Verify: `go build ./cmd/spinloop` succeeds.

## 2. Tests

- [x] 2.1 Update `TestBarGlyph` in `cmd/spinloop/metrics_render_test.go` to the seven-grade mapping — 0→`▁`, 12.5→`▁`, 30→`▃`, 50→`▄`, 75→`▆`, 87.4→`▇`, 87.5→`▇`, 100→`▇` — and verify `go test -run TestBarGlyph ./cmd/spinloop` passes.
- [x] 2.2 Add a test asserting `barGlyph` returns `▇` at 100 and never returns the full block `█` across the 0–100 range, covering the spec's "no sparkline glyph is a full block" scenario; verify it passes under `go test ./cmd/spinloop`.
- [x] 2.3 Update the glyph expectations in `TestRenderSparklineColoursOnlyTheLastPoint` (79.9→`▆`, 85→`▆`, 95→`▇`) and `TestRenderStatBarsDrawsHistory` (95→`▇`, 40→`▃`) in `cmd/spinloop/metrics_render_test.go`, and verify `go test ./cmd/spinloop` passes.
- [x] 2.4 Run `go test ./...` and fix any remaining assertions that pinned the old eight-grade sparkline mapping (e.g. the sparkline `Contains` checks in `cmd/spinloop/fleet_test.go`), leaving the gauge-based expectations (`dashBar` in `cmd/spinloop/fleet_dashboard_test.go`) untouched; verify the full suite is green.

## 3. Final checks

- [x] 3.1 Run `gofmt -l ./cmd/spinloop` (expect no files listed) and `go test ./... -cover` (expect total coverage >= 80%) and verify both hold.
