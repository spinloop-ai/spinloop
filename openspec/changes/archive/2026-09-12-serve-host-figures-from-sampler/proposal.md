## Why

The daemon's metrics endpoint collected the host's figures inline: reading CPU
costs a host command that takes longer the busier the host is, so on exactly the
machine someone has opened the dashboard to watch, the handler blocked past the
fleet client's timeout and the node rendered as unreachable while it was
answering fine. The figures were also collected twice — once by the background
sampler for the retained history, and again on every request.

## What Changes

- `/v1/metrics` reports the background sampler's last system reading — the
  GPU, CPU and memory figures — instead of taking a fresh collection on the
  request: one collection per tick serves both the retained history and the
  reported reading, and a reported figure is at most one sampling interval
  stale.
- A system reading that fails still becomes the reported reading, carrying its
  errors: with no collection taken on the request, the reading is the only
  place a broken source is reported from. The retained history still gets no
  figures for that tick.
- Before the first reading lands, the host figures are absent rather than a
  fresh collection or zero, and a start after a stop drops the previous
  engine's reading.
- The sampler's short catch-up interval now also requires a known scrape
  target: an engine whose runner exposes no metrics endpoint settles at the
  tick instead of running the host commands every second for counters that are
  never coming.
- The fleet dashboard polls the local daemons every 5 seconds rather than 2:
  one call per machine per tick, and the figures it draws are sampled every 15
  seconds anyway.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `daemon-api`: the metrics endpoint's host figures are the sampler's last
  reading, not a collection taken on the request; before the first reading they
  are absent, a start after a stop drops the previous reading, and a reading's
  collection failures are reported among the response's errors.
- `engine-activity`: one system reading per tick serves both the retained
  history and the reported reading; a failed reading gets no figures in the
  history but is still the reported reading, carrying its errors.

## Impact

- `internal/daemon`: the daemon keeps the most recent system reading (figures,
  the collection's failures, whether a reading has landed), recorded by the
  sampler and dropped on a start after a stop; the metrics handler copies the
  last reading instead of calling the collector; the catch-up interval applies
  only while a scrape target is known.
- `cmd/spinloop`: the dashboard's local refresh interval moves from 2 seconds
  to 5 seconds.
- No API-shape change: the same fields carry the figures, and the response's
  errors already existed, so `docs/openapi.yaml` is untouched. No `remote/`
  change, no new dependencies.
