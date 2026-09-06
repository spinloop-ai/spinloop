## Why

`spinloop remote metrics`, `spinloop fleet metrics`, and the fleet dashboard all draw utilisation as an instant block bar. That answers "how full is it right now" but nothing about how the level has been moving, which is what an operator actually asks when they watch an engine. None of the three surfaces can show a 0–100% series over time.

## What Changes

- **BREAKING**: `--format=bar` (the default) now draws a sparkline of recent samples per series — `CPU ▁▂▃▅▇▆▅▃▄▅▇ 62%` — instead of an instant filled bar. The previous filled-bar drawing is kept under a new name, `--format=gauge`.
- The daemon retains a rolling 10-minute history of system readings (CPU, RAM, each GPU's utilisation and memory), sampled at the existing 15-second sampler cadence while an engine runs. The history is exposed on `/v1/metrics`, persists across an engine stop, and is cleared when the next engine starts.
- The remote stats Lambda relays the daemon's history in its reply, so `spinloop remote metrics` gets the same view through the control plane.
- The fleet dashboard gains a `g` key that toggles every tile between bar and gauge; tiles open in bar.
- JSON output gains the history field (additive). `docs/openapi.yaml` gains it on the metrics response.
- Where no history is available (an older daemon), the bar format draws the current reading in the gauge style, so a pre-history daemon renders exactly as it does today.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `remote-metrics-bar-format`: `bar` becomes the history sparkline (its drawing, its colour rule, and its behaviour for a stopped engine); the previous drawing is added as the `gauge` format; the history source, window, and no-history fallback are specified.
- `remote-stats`: the format list gains `gauge`; the report carries the daemon's history where the control plane relays it.
- `fleet-client`: `fleet metrics` accepts `gauge`; the dashboard's panels default to the bar format and a `g` key toggles bar and gauge.
- `daemon-api`: the metrics endpoint includes the retained history of system readings.
- `engine-activity`: the background sampler takes a system reading on each tick while an engine runs, feeding the retained history.

## Impact

- Go: `internal/metrics` (stats shape), `internal/daemon` (sampler, ring buffer, `/v1/metrics`), `cmd/spinloop` (renderers, `--format` in remote.go and fleet.go, dashboard model and render), `docs/openapi.yaml`, and tests across those packages.
- TypeScript (`remote/`): shared types and the stats Lambda relay the history field; covered by the pnpm suite.
- CLI: `--format=bar` output changes for existing users, and so does the default output of both metrics commands; `--format=gauge` restores the previous drawing.
- Remote environments carry history only once the on-instance daemon is new (next boot or redeploy); until then they render the gauge fallback.
