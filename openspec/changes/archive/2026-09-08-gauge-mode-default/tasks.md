## 0. Spec bookkeeping

- [x] 0.1 Rename the two scenario titles in the main specs that encode the old default — "Default format is bar" to "Default format is gauge" in `openspec/specs/remote-stats/spec.md` ("Tabular display"), and "The board opens in bar" to "The board opens in gauge" in `openspec/specs/fleet-client/spec.md` ("Dashboard format toggle") — titles only; the content follows when the delta archives. Verify: `openspec validate gauge-mode-default` passes.

## 1. One-shot metrics commands

- [x] 1.1 In `cmd/spinloop/remote.go`, set the `remote metrics` `--format` flag default to `"gauge"` and its usage to `output format: gauge (default), bar, table or json`, and reword the `remoteMetricsCmd` doc comment so gauge is the default and `--format=bar` the history sparkline. Verify: `go build ./...` and `go run ./cmd/spinloop remote metrics --help` show gauge as the default.
- [x] 1.2 In `cmd/spinloop/remote_test.go`, update `TestRemoteMetrics_DefaultFormat` to give the fixture resource figures and assert the bare invocation renders the gauge drawing (filled gauge of the current reading, no sparkline glyphs), per the spec scenario "Default format is gauge". Verify: `go test ./cmd/spinloop -run TestRemoteMetrics_DefaultFormat`.
- [x] 1.3 In `cmd/spinloop/fleet.go`, set the `fleet metrics` `--format` flag default to `"gauge"` and its usage to `output format: gauge (default), bar, table or json`. Verify: `go build ./...` and `go run ./cmd/spinloop fleet metrics --help` show gauge as the default.
- [x] 1.4 In `cmd/spinloop/fleet_test.go`, point `TestCmdFleetMetricsFormats`' default subtest at gauge output (renaming it so it no longer claims "bar" is the default, and keeping an explicit `--format=bar` check for the bar drawing), and rework `TestCmdFleetMetricsDrawsHistory` so the bare invocation asserts the gauge format while the history-drawing assertions run under `--format=bar`. Verify: `go test ./cmd/spinloop -run 'TestCmdFleetMetrics'`.

## 2. Fleet dashboard

- [x] 2.1 In `cmd/spinloop/fleet_dashboard.go`, set `gauge: true` on the model `dashModelFor` returns, and update the `gauge` field comment in `dashboard_model.go` to say the board opens in gauge and that `dashModelFor` sets it. Verify: `go build ./...` and the assertion added in 2.2.
- [x] 2.2 In `cmd/spinloop/fleet_dashboard_test.go`, add `if !m.gauge` to `TestDashModelForFleetFile`'s successful `dashModelFor("")` result (the board opens in gauge, through the production construction path), and start `TestDashModelFormatToggle` from the production opening state (`gauge: true`) so its toggle asserts bar and back to gauge. Verify: `go test ./cmd/spinloop -run 'TestDashModelForFleetFile|TestDashModelFormatToggle'`.
- [x] 2.3 Reword the `fleet dashboard` long help in `fleet_dashboard.go` so the tiles are said to draw what the gauge format of `fleet metrics` prints. Verify: `go run ./cmd/spinloop fleet dashboard --help` names gauge, and no "bar format of fleet metrics" phrase remains in `cmd/spinloop`.

## 3. Docs

- [x] 3.1 Update `docs/commands/remote.md`, `docs/commands/fleet.md` (metrics section, dashboard section, and the `--format` flag table), and `docs/internals.md` so gauge is the named default and bar is the explicit/toggled one. Verify: `grep -rn 'bar (default)\|opens in bar\|opening in bar\|bar by default' docs/` returns nothing.

## 4. Verification

- [x] 4.1 Run `gofmt -l ./...` (clean), `go vet ./...`, and `go test ./... -cover`; confirm all tests pass and total coverage stays at or above 80%.
