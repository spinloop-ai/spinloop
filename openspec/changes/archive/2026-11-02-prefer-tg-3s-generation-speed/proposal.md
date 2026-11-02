# Proposal

## Why

llama.cpp's `tg` figure averages the whole generation task, so the dashboard's
Speed row lags badly when the rate changes — a model that just sped up (or
slowed under contention) keeps showing its old pace for minutes. llama.cpp now
also prints `tg_3s`, a three-second rolling rate that answers "how fast is it
generating *now*", which is exactly what the Speed row claims to show — but
today's parser deliberately reads only `tg` and never `tg_3s`.

## What Changes

- The engine-output speed parser reads `tg_3s` when the line carries it, and
  falls back to `tg` when it does not or when the `tg_3s` figure is not a
  usable rate. Engines whose llama.cpp predates `tg_3s` (and other engines
  printing `tg`) keep reporting speed unchanged.
- The rendered Speed row therefore reflects the recent rate rather than the
  since-task average. No change to rendering, folding, history, or lapse
  behaviour — only which figure on the line is read.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `engine-metrics`: "Generation speed from engine output" — the figure read
  becomes the three-second rate `tg_3s` where the line carries a usable one,
  falling back to `tg`; the scenario "An average is not the current rate"
  keeps its name but its rejected average becomes the whole-task `tg`
  instead of `tg_3s`.

## Impact

- `internal/metrics/speed.go` — one added `tg_3s` pattern and a fallback in
  `ParseGenerationSpeed`.
- `internal/metrics/speed_test.go`, `internal/daemon/speed_test.go` —
  expectations for lines carrying both figures, and the "average yields
  nothing" cases.
- `docs/maintainer/internals.md` — the paragraph pinning the parser to
  `tg`-only.
- No API, wire-format, or rendering changes: the stat's shape is unchanged.
