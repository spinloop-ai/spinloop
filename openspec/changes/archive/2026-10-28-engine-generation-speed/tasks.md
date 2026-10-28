# Tasks

## Shape the stat

- [x] Add `Speed *float64` (`json:"speed,omitempty"`) to `metrics.Stats` and `Speed *float64` (`json:"s,omitempty"`) to `metrics.HistorySample` in `internal/metrics/metrics.go`, with doc comments in the house style (absent-not-zero; `s` one-letter for the SSM budget)
- [x] Add `ParseGenerationSpeed(line string) (float64, bool)` to `internal/metrics` (parse.go or speed.go): `tg = <float> t/s`, never `tg_3s`, never prompt-eval rates, reject NaN/Inf/≤0; table-driven tests over the issue's real `print_timing` lines

## Capture it

- [x] Add the `lineTee` writer (buffer complete lines to `OnOutput`, size-capped partial buffer, bytes to the file first) and the `OnOutput func(string)` hook to the supervisor (`internal/daemon/supervisor.go`): stderr and the pty pump's output both write through it when `LogPath` is set and a hook is wired; tests for split writes, capped partial lines, and no-pty path
- [x] Wire `Daemon.StartEngine` to set `Sup.OnOutput` to the daemon's observer and reset the speed tracker/tick baseline beside the existing `forget`/`clear` calls

## Track and report it

- [x] Add `speedTrack` to the daemon (`internal/daemon/speed.go`): `observe` under the daemon clock; a consumed-point rule for the tick (`systemSampleOnce` attaches `sample.Speed` when a figure arrived since the last tick, and a speed point alone still makes the sample exist) and a fresh-within-one-interval rule for the current reading in `Daemon.Metrics`; tests for steady ticks, quiet ticks, figure latching then lapsing, start-after-stop reset

## Contract

- [x] Add `speed` to the `Stats` schema and `s` to the `HistorySample` schema in `docs/openapi.yaml`; `go test ./internal/daemon -run TestOpenAPI` green

## Render it

- [x] `cmd/spinloop/metrics_render.go`: relabel GPU utilisation series `GPU util`/`GPU N util` → `GPU`/`GPU N` in `barSeriesList`; add the `Speed` series (label `Speed`, watermark-scaled uncoloured sparkline via a speed glyph block that pools then scales, `▇` ceiling, `%6.2f t/s` figure) drawn after the resource series in `renderStatBars` and `renderStatCombined` (blank gauge half); `renderStatGauges` and the table format carry the figure line only; omit the row when neither reading nor history carries speed
- [x] Thread the speed through the callers: `fleet.go` (bar/gauge/table), `dashboard_render.go` (tile body), `serve_view.go` (metrics section)
- [x] Update the golden tests the relabel and new row touch (`metrics_render_test.go`, `metrics_render_gpu_test.go`, `fleet_test.go`, `fleet_dashboard_test.go`, serve view tests), adding cases for the issue's example window: watermark mapping, default colour at every height, absent-speed omission

## Verify

- [x] `gofmt -w`, `go vet ./...`, `go test ./... -cover` green, coverage ≥ 80%

## Layout follow-up (from review of the rendered view)

- [x] Tighten the block's shared label column to its longest label (`statLabelWidth`: "GPU mem" = 7, widening by the GPU index); thread the width through `renderSparkline`/`renderGauge`/`renderActiveIndented` and the Speed rows
- [x] Serve view: the speed figure stands at the graph column inside a fixed field spanning the gauge and percentage halves, pinning the sparkline to the sparklines' column across digit counts
- [x] Bar format: sparkline in the bars' column, t/s figure trailing with the other rows' percentages; gauge/tile rows carry the figure at the graph column
- [x] A lapsed figure on an engine that reports one draws `0.00 t/s` wherever the row shows (`speedShown`/`speedFigure`); only a never-reporting engine has no row
- [x] `dashBarLineW` 25→23 so the trailing t/s figure fits the 42-column tile; goldens and spec/design/docs updated

## Sparkline lapse follow-up (from review)

- [x] Place speed points on the window's time axis (`speedSeries` one slot per sample, `poolMaxGaps` pooling without closing gaps) so a lapse empties the sparkline's right edge beside its `0.00 t/s` figure instead of pinning the last burst to it
