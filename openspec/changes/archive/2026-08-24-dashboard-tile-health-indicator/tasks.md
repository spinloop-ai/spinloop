## 1. Engine readiness check

- [x] 1.1 Add `CheckEngineReady(ctx context.Context, target ScrapeTarget) bool` to `internal/metrics/scrape.go`, mirroring `ScrapeTokenStats`'s URL handling (trim `/v1` suffix, hit `/health` at the server root) but with no `Authorization` header — an unauthenticated `200` or `401` both mean ready; any other status, a request error, or a malformed base URL means not ready.
- [x] 1.2 Add a `readiness` guarded struct to `internal/daemon` (new file `readiness.go`), mirroring `engineSample`'s shape: `record(ready bool)`, `read() (ready, have bool)`, `forget()`.
- [x] 1.3 Add `readinessCheckedRunners` — a `map[string]bool{"llamacpp": true, "vllm": true}` — in `internal/daemon/readiness.go`, documenting that a runner not listed has no established health-check convention and is left unchecked.
- [x] 1.4 Add `Daemon.checkReadyOnce(ctx)`: returns early when the engine is not `StateRunning`, when `d.scrape.BaseURL` is empty, or when `d.scrape.Engine` is not in `readinessCheckedRunners`; otherwise copies `d.scrape` under the lock (released before the network call, matching `sampleOnce`) and calls `metrics.CheckEngineReady`, recording the result on `d.ready`.
- [x] 1.5 Call `d.checkReadyOnce(ctx)` from within `SampleActivity`'s existing per-tick loop in `internal/daemon/activity.go`, alongside `d.sampleOnce(ctx)`, so it rides the same ticker and catch-up cadence rather than starting a second background loop.
- [x] 1.6 Add `d.ready.forget()` to `StartEngine` in `internal/daemon/daemon.go`, alongside the existing `d.sample.forget()`, so a previous engine's readiness is never attributed to the one that replaced it.

## 2. Report readiness on status and metrics

- [x] 2.1 Add `Ready string` (json tag `ready,omitempty`) to `StatusResponse` in `internal/daemon/daemon.go`, documented as `"ready"`, `"not-ready"`, or absent (engine not running, runner unchecked, or daemon predates this field).
- [x] 2.2 Add the same `Ready string` field to `metrics.Stats` in `internal/metrics/metrics.go`, with a doc comment cross-referencing `StatusResponse.Ready` the same way `LastActiveAt`/`IdleSeconds` cross-reference each other.
- [x] 2.3 Add a `Daemon.readinessField() string` helper reading `d.ready.read()` and rendering `"ready"`/`"not-ready"`/`""`; call it from both `Status()` and `Metrics()` in `internal/daemon/daemon.go`, inside each function's existing `if state == StateRunning` block (matching how the `Engine` field is already scoped).
- [x] 2.4 Update `docs/openapi.yaml`: add a `ready` property (`type: string`, `enum: [ready, not-ready]`) to both the `StatusResponse` and `Stats` schemas, with a description matching the Go doc comments.
- [x] 2.5 Run `go test ./internal/daemon/... -run OpenAPI` to confirm the contract test passes against the updated schema.

## 3. Compute a tile's health tier

- [x] 3.1 Add a `dashHealthTier` type with four values (e.g. `dashHealthy`, `dashAttention`, `dashUnhealthy`, `dashUnknown`) in `cmd/outfit/dashboard_render.go`.
- [x] 3.2 Add `dashHealthTierFor(r fleet.NodeResult, a dashAction) dashHealthTier`: an in-flight action (`a.verb != ""`) is `dashAttention`; else no refresh yet (`r.Outcome == ""`) is `dashUnknown`; else a failed outcome (`!r.OK()`) or `r.Metrics.State == "crashed"` is `dashUnhealthy`; else `r.Metrics.State == "running" && r.Metrics.Ready == "not-ready"` is `dashAttention`; else an answer carrying no state at all (`r.Metrics.State == ""`) is `dashUnknown`; else `dashHealthy`.
- [x] 3.3 Add `dashHealthGlyph(tier dashHealthTier) string` returning a glyph wrapped in the matching raw ANSI colour — `"●"` for the known tiers, `?` for unknown — reusing `renderBar`'s palette (green `\033[92m`, yellow `\033[33m`, red `\033[31m`, grey `\033[90m`, reset `\033[0m`).

## 4. Render the glyph on every tile shape

- [x] 4.1 In `dashTileContent`, compute the glyph once via `dashHealthTierFor`/`dashHealthGlyph` before the shape switch.
- [x] 4.2 Prepend `"<glyph> "` to the name in each of the four shapes' first line (action in flight, no refresh yet, failed outcome, settled answer), matching the `● name  ...` layout from the proposal.

## 5. Update tests and verify

- [x] 5.1 Add daemon-level tests in `internal/daemon/*_test.go`: a running engine whose fake health endpoint answers `200` reports `Ready: "ready"` on both `Status()` and `Metrics()`; one answering `503`/refused reports `"not-ready"`; a runner not in `readinessCheckedRunners` reports `Ready: ""`; an idle/stopped/crashed engine reports `Ready: ""`; a fresh `StartEngine` after a previous ready reading clears it until the next check lands.
- [x] 5.2 Add a test in `internal/metrics` for `CheckEngineReady`: `200` and `401` both return `true`; `503`, connection-refused, and a malformed base URL all return `false`.
- [x] 5.3 Update `TestDashTileRunningByteStable`, `TestDashTileOutcomeAndEmpty`, `TestDashTileStoppedByteStable`, `TestDashTileActionInFlight`, and `TestDashTileActionInFlightWithReport` in `cmd/outfit/fleet_dashboard_test.go` to expect the glyph and its colour on each fixture's first line.
- [x] 5.4 Add a test asserting the four-tier-input mapping directly (table-driven over representative `fleet.NodeResult`/`dashAction` combinations): `running`+`Ready: "ready"` → healthy; `running`+`Ready: "not-ready"` → attention; `running`+`Ready: ""` → healthy (degrade gracefully); `crashed` → unhealthy; each of `unreachable`/`unauthorized`/`config-error`/`failed`/`unsupported` outcomes → unhealthy; no outcome yet → unknown; an answered result carrying no state → unknown; action in flight over a healthy report → attention regardless of the report.
- [x] 5.5 Add or extend a test confirming the glyph colour is unaffected by `selected` (border colour changes, glyph colour does not) — covers the spec's "glyph is distinct from the selection border" scenario.
- [x] 5.6 Run `gofmt -l cmd/outfit internal/daemon internal/metrics` and `go test ./cmd/outfit/... -run Dash -v` to confirm formatting and all dashboard tests pass.
- [x] 5.7 Run `go test ./... -cover` to confirm the full suite still passes and coverage stays >= 80%.
