## Why

The retention figure counts down in seconds — `keep for 2h 5m 30s`. In the
dashboard, where it re-renders on every refresh, the seconds component churns
without meaning anything: a keep is set in minutes or hours, and nobody acts
on the difference between `keep for 59s` and `keep for 30s`. It also spends
tile width on a unit the figure does not need.

## What Changes

- The relative retention figure that every stats surface renders — the
  dashboard tile, its detail screen, and the one-shot `fleet metrics` and
  `remote metrics` reports — uses minute granularity: hours and minutes, zero
  units dropped (`keep for 2h`, `keep for 24m`, `keep for 1h 30m`), and never
  a seconds component.
- A remaining time of less than a minute still renders — as `keep for 1m` —
  so the figure disappears only when the deadline itself passes, under the
  same omission rule as today.
- The elapsed-time figure beside it (`active  2m 5s ago`) is untouched: it
  keeps its seconds.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `remote-stats`: "The stats reply carries the retention deadline" — the
  wording of the deadline's relative remaining time gains a granularity rule:
  hours and minutes only, no seconds. The dashboard tile needs no delta of its
  own: `fleet-client` already requires it to be worded the same way the
  one-shot surfaces report it.

## Impact

- `cmd/spinloop/metrics_render.go`: `formatKeepDuration`, the one renderer
  behind the keep figure on every surface (the tile and detail screen via
  `renderActiveIndented`, the table format via `renderActiveKeyValue`, and the
  one-shot remote report). No other call site renders the figure.
- Tests: `cmd/spinloop/retain_render_test.go` and
  `cmd/spinloop/dashboard_keep_test.go` pin the rendered figure; the figures
  they assert already carry no seconds, so they stand, and coverage for the
  sub-minute case is added.
- No API, control-plane, or data-shape change: the deadline still travels as
  an absolute RFC 3339 timestamp, and the `json` formats carry that timestamp,
  not the rendered figure. No doc example shows seconds in the figure, so
  `docs/` stands as well.
