# Tasks

## Shape the stat

- [ ] Add `Speed *float64` (`json:"speed,omitempty"`) to `metrics.Stats` and `Speed *float64` (`json:"s,omitempty"`) to `metrics.HistorySample` in `internal/metrics/metrics.go`, with doc comments in the house style (absent-not-zero; `s` one-letter for the SSM budget)
- [ ] Add `ParseGenerationSpeed(line string) (float64, bool)` to `internal/metrics` (parse.go or speed.go): `tg = <float> t/s`, never `tg_3s`, never prompt-eval rates, reject NaN/Inf/≤0; table-driven tests over the issue's real `print_timing` lines

## Capture it

- [ ] Add the `lineTee` writer (buffer complete lines to `OnOutput`, size-capped partial buffer, bytes to the file first) and the `OnOutput func(string)` hook to the supervisor (`internal/daemon/supervisor.go`): stderr and the pty pump's output both write through it when `LogPath` is set and a hook is wired; tests for split writes, capped partial lines, and no-pty path
- [ ] Wire `Daemon.StartEngine` to set `Sup.OnOutput` to the daemon's observer and reset the speed tracker/tick baseline beside the existing `forget`/`clear` calls

## Track and report it

- [ ] Add `speedTrack` to the daemon (`internal/daemon/speed.go`): `observe` under the daemon clock; a consumed-point rule for the tick (`systemSampleOnce` attaches `sample.Speed` when a figure arrived since the last tick, and a speed point alone still makes the sample exist) and a fresh-within-one-interval rule for the current reading in `Daemon.Metrics`; tests for steady ticks, quiet ticks, figure latching then lapsing, start-after-stop reset

## Contract

- [ ] Add `speed` to the `Stats` schema and `s` to the `HistorySample` schema in `docs/openapi.yaml`; `go test ./internal/daemon -run TestOpenAPI` green

## Render it

- [ ] `cmd/spinloop/metrics_render.go`: relabel GPU utilisation series `GPU util`/`GPU N util` → `GPU`/`GPU N` in `barSeriesList`; add the `Speed` series (label `Speed`, watermark-scaled uncoloured sparkline via a speed glyph block that pools then scales, `▇` ceiling, `%6.2f t/s` figure) drawn after the resource series in `renderStatBars` and `renderStatCombined` (blank gauge half); `renderStatGauges` and the table format carry the figure line only; omit the row when neither reading nor history carries speed
- [ ] Thread the speed through the callers: `fleet.go` (bar/gauge/table), `dashboard_render.go` (tile body), `serve_view.go` (metrics section)
- [ ] Update the golden tests the relabel and new row touch (`metrics_render_test.go`, `metrics_render_gpu_test.go`, `fleet_test.go`, `fleet_dashboard_test.go`, serve view tests), adding cases for the issue's example window: watermark mapping, default colour at every height, absent-speed omission

## Verify

- [ ] `gofmt -w`, `go vet ./...`, `go test ./... -cover` green, coverage ≥ 80%
