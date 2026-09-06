## Context

All three metrics surfaces (`remote metrics`, `fleet metrics`, the fleet dashboard) render a point-in-time snapshot; no history is kept anywhere, client or server. The daemon already runs a background sampler (`SampleActivity`, `internal/daemon/activity.go`) that ticks at `DefaultSampleInterval` (15s) while an engine runs, but it reads only the engine's token counters and only when a scrape target is known. System stats (CPU/RAM/GPU via `metrics.Collector.System`) are collected on demand inside `Daemon.Metrics()`. The remote stats Lambda relays the daemon's `/v1/metrics` reply over an SSM curl and copies fields into its own reply — and SSM command output is truncated at 4KB.

## Goals / Non-Goals

**Goals:**
- A 0–100% sparkline of the last 10 minutes per resource series on all three surfaces, selectable as `bar` (the default), with the previous filled-bar drawing preserved as `gauge`.
- History owned by the daemon so every client — one-shot, watch, dashboard, remote relay — sees the same window from a single read.

**Non-Goals:**
- Persisting history to disk, or longer than a 10-minute window.
- Charting non-percentile series (token counters stay as plain lines; they have no 0–100% axis).
- A per-node format choice in the dashboard, or charting in the dashboard's detail view.

## Decisions

### 1. The daemon owns the history; clients never accumulate

A ring buffer in the daemon, exposed on `/v1/metrics`, beats client-side accumulation: a one-shot `--format=bar` shows real history on first read, the dashboard needs no per-client buffer, and every client sees the same window. Client accumulation was rejected because a one-shot would degenerate to a single point and each client would show a different window.

### 2. The sampler takes a system reading each tick

`SampleActivity` already ticks at 15s while the engine runs. Each tick additionally runs the system collector and appends one reading to the buffer. This is deliberately independent of the counter scrape's gates: system figures come from host commands, not from the engine's metrics endpoint, so a scrape target is not required — only the engine being running (a stopped engine has no utilisation to chart, matching the bar format's existing behaviour).

A failed system reading records no sample and reports no error — the same "a failed sample is a non-observation" rule the counter sampler follows, and the on-request collection keeps its own error reporting.

Retention hooks follow the existing engine-boundary hooks in `StartEngine`: the buffer is cleared there alongside `sample.forget()` (one engine's readings never meet the next engine's), and nothing clears it on stop, so the history persists across a stop exactly like the last-active record.

### 3. Samples store percentages, not raw figures

Each sample carries the time (unix seconds) and, per series, the 0–100% figure the sparkline plots: CPU utilisation, RAM used/total, and per-GPU utilisation and memory. Storing raw bytes would double the wire size for nothing the graph uses, and the 4KB SSM relay budget makes the difference concrete: 40 samples (10 min at 15s) of percentages is roughly 2–2.5KB; raw memory figures push it toward the truncation limit. The current reading keeps its existing raw shape; only history is percentage-form.

The Go shape lives in `internal/metrics` (the shared stats dialect): a `History` slice on `Stats`, each sample with `time`, `cpu`, `mem`, and `gpus` (index, util, mem), all omitted when absent — the existing absence-not-zero convention.

### 4. Rendering: eight block glyphs, max-pooled, last point coloured

The sparkline maps each value to one of eight Unicode block elements (U+2581–U+2588). Where the window holds more samples than the draw width, each column takes the **maximum** of its pool: utilisation thresholds are about peaks, and a max keeps a red spike visible that an average would smooth away. The final glyph takes the bar format's green/yellow/red colour by threshold; earlier glyphs use the terminal default.

The full view draws one glyph per sample (up to the 40-column window); the 42-column dashboard tile downsamples to fit, which keeps both formats at one line per series — the tile layout does not change.

### 5. No-history fallback is the gauge drawing

Where a daemon reports no history (predates the feature, or an engine with no reading yet), bar format draws the current reading in the gauge's filled style. Consequence worth stating: a fleet mixed with older daemons renders those nodes exactly as it does today, per node, with no client-side version detection.

### 6. Code rename: the old `renderBar` becomes the gauge renderer

`bar` now names the sparkline format, so `renderBar`/`renderStatBars` are renamed to their gauge role and the sparkline paths take the `bar` names. `--format` on both metrics commands accepts `bar|gauge|table|json`; the dashboard carries a board-wide format state toggled by `g`, opening in bar.

### 7. The Lambda relays the field verbatim

`shared/daemon.ts` and `shared/stats.ts` gain the history types, and the stats Lambda copies the field through unchanged — the same treatment as `lastActiveAt`/`idleSeconds`: the daemon decides what counts, the relay does not reshape it.

## Risks / Trade-offs

- [SSM output truncation at 4KB] → samples are percentage-form and the window is fixed at 40; the reply stays well under the limit. If the window or sample shape ever grows, the daemon-side size is the place to cap it, not the client.
- [Unconditional 15s system sampling while an engine runs] → one set of host commands (`nvidia-smi`/`vmstat` or `top`+`vm_stat`) every 15s even when nobody is watching. Small and bounded; the same commands already run on every metrics request, and the dashboard polls those every 2s today.
- [Default output changes for existing users] → `--format=gauge` restores the previous drawing; the release notes name it.
- [Remote environments gain history only after the on-instance daemon is new] → until the next boot/redeploy they render the gauge fallback; nothing breaks.
- [Max-pooling can overstate a spiky series' average level] → acceptable: the graph's job is to show pressure, and the trailing percentage is always the exact latest value.

## Migration Plan

Single release; the API change is additive (new field on `/v1/metrics`), and older daemons keep working through the per-node fallback. Rollback is a revert. No data migration.

## Open Questions

None — the window (10 min at the 15s cadence), the rename target (`gauge`), the toggle key (`g`), and the stopped-engine behaviour (retained buffer) were settled with the user.
