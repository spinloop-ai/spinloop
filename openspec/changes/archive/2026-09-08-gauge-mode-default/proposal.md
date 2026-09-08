## Why

Every metrics display in spinloop — `spinloop remote metrics`, `spinloop fleet metrics`, and the fleet dashboard — opens on the bar format, a sparkline of the daemon's retained history. The question an operator asks most often when watching an engine is "how full is it right now", and the gauge format answers that directly while the bar format answers "how has it been moving". The current reading should be what the default shows; the history is one flag or one key away.

Note: the request this change was made from spells the format "guage"; the format in the code and specs is `gauge`, and this change uses that name throughout. No format called `guage` is introduced.

## What Changes

- **BREAKING**: `spinloop remote metrics` without `--format` renders gauge format (the current reading as filled progress gauges) instead of bar format. `--format=bar` restores the previous default display, exactly as it is drawn today.
- **BREAKING**: `spinloop fleet metrics` without `--format` renders gauge format per node instead of bar format, with the same `--format=bar` opt-out.
- **BREAKING**: the fleet dashboard board opens in gauge format instead of bar; `g` still toggles every panel between the two.
- The `--format` flag usage on both metrics commands and the `fleet dashboard` help text now name gauge as the default.
- The bar format's no-history fallback (draw the current reading in the gauge's filled style) is unchanged in behaviour; its stated rationale no longer rests on bar being the default.
- Unchanged: the `serve` full-screen view, which draws each series in both formats at once and has no default to flip; the set of formats and their validation; the table and json formats.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `remote-metrics-bar-format`: "Bar format is default" is removed and replaced by "Gauge format is default"; the no-history fallback in "Bar draws the retained history" drops the clause that restated bar as the default.
- `remote-stats`: "Tabular display" lists gauge first as the default; "History in the report"'s no-history scenario is pinned to `--format=bar`, since a bare invocation no longer draws the bar fallback.
- `fleet-client`: "Fleet metrics" renders gauge per node without `--format` (bar becomes the explicit case); "Dashboard panels show the node's metrics" says the board's current format is gauge by default; "Dashboard format toggle" says the board opens in gauge.

## Impact

- Go (`cmd/spinloop`): the `--format` flag default and usage string in `remote.go` and `fleet.go`; the dashboard model's opening format and the `fleet dashboard` long help; tests in `remote_test.go`, `fleet_test.go`, and `fleet_dashboard_test.go` that assert the old defaults.
- Docs: `docs/commands/remote.md`, `docs/commands/fleet.md`, `docs/internals.md` name the default in several places.
- No API, daemon, or control-plane change: the format is a client-side choice, and the remote/ TypeScript project is untouched.
- Users: the default one-shot output of both metrics commands changes, and the dashboard opens in the other format; every previous display is reachable with `--format=bar` or one press of `g`.
