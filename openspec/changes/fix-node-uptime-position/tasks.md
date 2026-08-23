## 1. Render the state line with uptime

- [x] 1.1 Add a helper (e.g. `dashStateLine(state string, m metrics.Stats) string`) in `cmd/outfit/dashboard_render.go` that returns `state` alone, or `state  (up <duration>)` when `m.UptimeSeconds > 0`, reusing `formatDuration`.
- [x] 1.2 In `dashTileContent`'s settled `default` case, use the helper so the top line becomes `name  <dashStateLine(...)>` instead of `name  state`.
- [x] 1.3 In `dashTileContent`'s in-flight case (`a.verb != ""`, `r.OK()`), use the helper for the standalone state line instead of printing `r.Metrics.State` directly.

## 2. Drop uptime from the serving line

- [x] 2.1 Remove the `UptimeSeconds` block from `dashTileServingLine` in `cmd/outfit/dashboard_render.go`, leaving just runner and model ID.
- [x] 2.2 Update `dashTileServingLine`'s doc comment to describe the new output (runner and model only).

## 3. Update tests and verify

- [x] 3.1 Update `TestDashTileRunningByteStable` in `cmd/outfit/fleet_dashboard_test.go`: move `(up 2h 0m 0s)` from the serving line onto the `"up  running"` top line, and drop it from the serving line.
- [x] 3.2 Update `TestDashTileActionInFlightWithReport`: move `(up 4m 0s)` from the `"vllm  org/qwen3:32b  (up 4m 0s)"` serving line onto the standalone `"running"` state line.
- [x] 3.3 Update `TestDashGridRealTilesSideBySide` and any other fixture in `cmd/outfit/fleet_dashboard_test.go` asserting a `(up ...)` line, to match the new layout.
- [x] 3.4 Add or extend a test with a long runner/model ID (long enough to have previously clipped the uptime) asserting the uptime now still renders, on the state line, within the 42-column tile width.
- [x] 3.5 Run `gofmt -l cmd/outfit` and `go test ./cmd/outfit/... -run Dash -v` to confirm formatting and all dashboard tests pass.
- [x] 3.6 Run `go test ./... -cover` to confirm the full suite still passes and coverage stays >= 80%.
