# Proposal

## Why

Operators watching a serving engine have no idea how fast it is actually generating tokens. llama.cpp already prints the figure in its structured log lines (`slot print_timing: ... tg = 3.87 t/s, tg_3s = 5.29 t/s`), but spinloop throws it away — the metrics views show host pressure (CPU/RAM/GPU) and cumulative counters, never the throughput the machine exists to produce. Surfacing it turns "is this engine any good?" from a guess into a glance.

## What Changes

- The daemon observes the generation speed (`tg`) in the engine's captured output lines, parses it, and reports it as a new optional metric: a current reading on the stats reply plus one point per sampling tick in the retained history. Engines that never print the figure report neither, on the usual "absent, not zero" terms.
- Every surface that renders the resource series gains a `Speed` row — `Speed  12.34 t/s  ▂▃▂▂▂▁▂▂▁▂▇▇▃▂▂` — with a sparkline whose glyph heights scale to the high watermark of the values currently shown in the window, and whose points all draw in the terminal's default colour (the 80/90% threshold colours are utilisation's, not speed's). The gauge and table formats carry the figure without a sparkline; JSON carries both new optional fields.
- The GPU utilisation series is relabelled: `GPU util`/`GPU N util` become `GPU`/`GPU N` (the memory series keeps `GPU mem`/`GPU N mem`).

## Capabilities

### New Capabilities

_None — all behaviour extends existing specs._

### Modified Capabilities

- `engine-metrics`: engine stats collection gains generation speed parsed from the engine's captured output, optional on the same terms as every other stat.
- `engine-activity`: the sampling tick folds a fresh speed observation into the retained history; a tick with no generation since the last one carries no speed point.
- `daemon-api`: `/v1/metrics` reports the current speed reading (while the engine runs) and speed points in the history, both optional.
- `remote-metrics-bar-format`: bar/gauge/table formats draw the `Speed` row (sparkline scaled to the window's high watermark, default colour throughout, figure in t/s); GPU utilisation series relabelled from `GPU util`/`GPU N util` to `GPU`/`GPU N`.

## Impact

- `internal/metrics` — parser for the engine's timing line; `Stats.Speed` and `HistorySample.Speed` fields.
- `internal/daemon` — supervisor output pipeline gains a line observer; the daemon tracks speed and folds it at the sampling tick; `/v1/metrics` reply shape.
- `docs/openapi.yaml` — new optional properties (policed by `openapi_test.go`).
- `cmd/spinloop/metrics_render.go` (+ serve view, dashboard, fleet/table callers) — Speed row rendering, sparkline scaling, GPU label rename; affected golden tests.
- Backward/forward compatible: old daemons and the TypeScript stats Lambda simply omit the fields; older fleet clients ignore them.
