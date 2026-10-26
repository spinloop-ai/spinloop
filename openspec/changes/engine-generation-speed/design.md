# Design

## Context

The daemon already captures every line the engine writes (stderr straight to the log file; stdout through the pty normaliser) and already runs a sampler tick (`SampleActivity`/`systemSampleOnce`) that builds the retained `HistorySample` window and the current host reading the metrics endpoint answers from. llama.cpp prints its instantaneous generation rate (`tg = 3.87 t/s`) into stderr's `print_timing` lines, which today reach the log file and stop there. The stats shape (`internal/metrics.Stats`, `HistorySample`) is a wire contract with the fleet client and the TypeScript control plane, policed against `docs/openapi.yaml` by `openapi_test.go`.

## Goals / Non-Goals

**Goals:**
- Surface the engine's reported generation speed as an optional current reading and an optional per-tick history point, end to end: engine output → daemon → `/v1/metrics` → bar/combined/tile/gauge/table rendering and the serve view.
- Sparkline heights relative to the window's own high watermark; default colour throughout.
- Relabel the GPU utilisation series `GPU util` → `GPU`.

**Non-Goals:**
- Computing speed from token-counter deltas ourselves (the engine's own figure is the spec'd source; a derived figure would have to define what a "generation interval" is across concurrent slots).
- vLLM/oMLX speed support (no `tg`-style line to parse; their absence follows the usual absent-stat path).
- Historical speed across restarts, or a persisted window beyond the existing 10-minute buffer.

## Decisions

**Parse where the bytes already become lines.** A `lineTee` writer wraps the supervisor's log-file sink: raw bytes flow to the file untouched; complete lines are additionally handed to an injected `Supervisor.OnOutput func(line string)` hook. One tee covers both capture paths — stderr (where `print_timing` lands) is written through it, and the pty pump's normalised output is written through it. The alternative — a log-file tailer that re-reads what was just written — duplicates the offset bookkeeping, sees bytes the pump may not have committed yet, and costs a second reader per daemon for a figure that is already in memory. `StartEngine` sets `Sup.OnOutput` to the daemon's own observer beside each start, so both API hosts (daemon and serve --api/view) wire it without per-command plumbing; `OnOutput` is therefore the daemon's property once a supervisor has a daemon. A partial line is buffered with a size cap (64 KiB) — a runaway line drops the buffer rather than growing it — because log lines worth parsing are ~100 bytes.

**Parser in `internal/metrics`, engine-shaped regex.** `ParseGenerationSpeed(line) (float64, bool)` matches `tg\s*=\s*<float>\s*t/s`: deliberately NOT matching `tg_3s =` (the underscore breaks the `\s*=` that must follow `tg`) nor prompt-evaluation rates (different label). Reject NaN/Inf/negative — llama.cpp prints `-nan` before a slot's first decode — and require a plausible rate (`> 0`). Engines that never print the line simply never yield a speed; no engine-family gating, so a future engine with the same spelling works for free.

**One tracker, two lifetimes.** A `speedTrack` on the daemon (mutex, last value, last-observed time) answers two questions with different clocks: the sampler tick attaches a history point when a figure arrived since the previous tick (points are observations, never continuations — a quiet tick's sample just lacks the field); the metrics handler answers the current reading only while the engine runs AND the last figure is no older than the sampling interval (spec: "since the last sampling tick"). Both read through the daemon's injected clock, so tests age the figure without sleeping. `StartEngine` resets the tracker, the tick baseline, and the history together, so one engine's rate never rides the next engine's report.

**Speed is a `barSeries`, drawn by its own rule.** The series list gains a trailing `Speed` series built from the current figure and the history's speed points; `renderStatBars`/`renderStatCombined` draw it with a speed-specific glyph block — value scaled to the window maximum (pool-then-scale, so downsampling keeps the watermark honest) mapping to `▇`, never `█` — and always uncoloured, because the 80/90 thresholds only mean something on a 0–100 axis; a rate has no ceiling. The figure renders `%6.2f t/s`. `renderStatGauges` draws the label and figure with no gauge block. The combined serve-view row keeps its bar half and leaves the gauge half blank; the table format gets a `speed:` key-value line beside the token counters. Callers keep taking explicit args from `metrics.Stats` rather than restructuring signatures: the existing surfaces already thread `(cpu, mem, gpus, history)` and gain `(speed)`.

**GPU relabel is one string.** `barSeriesList` builds `"GPU %d util"` today; the util series becomes `"GPU"` for one GPU, `"GPU N"` for many, the memory series unchanged. Every surface draws from this list, so tiles, detail, bar, gauge and serve view move together.

**Wire shape.** `Stats.speed *float64` and `HistorySample.s *float64` (one-letter, for the SSM 4KB budget the history fields already respect), both `omitempty`; `docs/openapi.yaml` gains both properties — `TestOpenAPISchemasMatchGoTypes` fails otherwise. Consumers that never see the field (older CLIs, the TS Lambdas) are unaffected.

## Risks / Trade-offs

- **Dashboard tiles are fixed-height** (`dashTileH`): a node with GPU series and a running engine may clip the token counter block when the Speed row joins. Accepted — the detail view is unclipped, and clipping keeps the tile grid honest; flag in review if the clip reads badly in practice.
- **Only engines that self-report are covered.** vLLM's `/metrics` exposes no instantaneous rate; its users see no row. Deriving speed from counter deltas is the fallback if demand appears.
- **Tee adds a copy path on the engine's output.** The copy is one `Write` of already-buffered bytes to an in-memory line splitter; the pump never blocks on the parser (regex over ~100 bytes), and a wedged observer cannot back-pressure the engine because the tee writes the file first and the hook after.
- **`--watch`/poll staleness:** between sampler ticks the current figure is up to one interval old — bounded staleness is already the documented model for every other metric in the reply.
