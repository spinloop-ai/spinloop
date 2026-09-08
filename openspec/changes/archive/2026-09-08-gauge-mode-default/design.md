## Context

The resource-series format is a purely client-side choice; the daemon and the
cloud control plane report facts and the CLI draws them. Three surfaces pick a
format:

- `spinloop remote metrics` and `spinloop fleet metrics` read it from their
  `--format` flag, whose default lives in the pflag registration
  (`fs.StringVar(&format, "format", "bar", ...)` in `remote.go` and
  `fleet.go`).
- The fleet dashboard holds the board's format in `dashModel.gauge`
  (`false` = bar, `true` = gauge), toggled board-wide by `g`. The production
  model is built in one place, `dashModelFor` (`fleet_dashboard.go`), and its
  field comment records that the zero value opens the board in bar.

The renderers already accept either format — `renderStatBars`,
`renderStatGauges`, the per-series gauge fallback — so nothing about drawing
changes; only which format each surface picks when told nothing.

## Goals / Non-Goals

**Goals:**

- Gauge is the format every metrics display draws when given none: both
  one-shot `--format` flags and the dashboard's opening board.
- Bar stays fully reachable: `--format=bar` and one press of `g`.
- Help text, key help, docs, and tests agree with the new default.

**Non-Goals:**

- No change to the formats themselves, their validation, or the `serve`
  full-screen view (it draws both at once and has no default).
- No daemon, control-plane, or `remote/` TypeScript change.

## Decisions

**1. Flip the flag default at its registration, not downstream.**
Both commands' `--format` flags get default `"gauge"`, and their usage strings
read `output format: gauge (default), bar, table or json`. `validateMetricsFormat`
and the renderers are untouched. Alternative considered: defaulting downstream
(e.g. a "unset" sentinel) — rejected; the flag already has exactly two
formats to choose between and pflag's zero value is the idiomatic place for it.

**2. The dashboard keeps its `gauge` field; the construction site sets it.**
`dashModelFor` sets `gauge: true` on the model it returns, and the field
comment says the board opens in gauge. The `g` toggle and every renderer read
the field as they do today. Alternative considered: renaming the field to
`bar bool` so the zero value meant gauge — rejected; `bar == false` meaning
gauge is a double negative at every read site, and the board's format is read
by the tile and detail renderers, so the churn would buy nothing over a one
line change in the one place a real board is built. Test models that build
`dashModel` literals keep their zero value (bar) and set `gauge: true` where
they want the board's real opening format.

**3. Two stale scenario titles are renamed in the main specs, by hand.**
The delta tooling treats a scenario's title as its identity: a MODIFIED
requirement must repeat every scenario title the main spec currently has, so
a title cannot be renamed through a delta. Two titles encode the old default
and would read as false once gauge is it: "Default format is bar" (remote-stats,
"Tabular display") and "The board opens in bar" (fleet-client, "Dashboard
format toggle"). Both are renamed directly in `openspec/specs/` — a title-only
edit, the same kind of main-spec edit the tooling allows for a Purpose — and
the delta blocks carry the new titles with the new behaviour. No scenario is
dropped; the rename is visible in the diff. The other affected scenario titles
("Bar format per node", "Gauge format per node", "Bar format is default" as a
requirement name) stay accurate or move with their requirement.

**4. Help and docs are updated in the same change.**
The `fleet dashboard` long help says each tile draws "what the bar format of
fleet metrics prints"; it will say gauge. `docs/commands/remote.md`,
`docs/commands/fleet.md`, and `docs/internals.md` name the default in several
places and are updated to match, so the shipped help and the docs cannot
disagree.

## Risks / Trade-offs

- [Scripts that consume the default output of `remote metrics` or
  `fleet metrics` will parse gauges where they parsed sparklines] → the change
  is marked BREAKING in the proposal; `--format=bar` reproduces the previous
  output byte for byte, and the flag was always the documented way to choose.
- [A test that builds a zero-value `dashModel` could be mistaken for covering
  the board's opening format when it covers bar] → the spec scenario "The
  board opens in gauge" is tested through `dashModelFor`, the production
  construction path, not a zero-value literal.
- [Help text and docs drift from the code if one is missed] → the tasks list
  every site; the suite asserts the rendered output, and `go vet`/build catch
  the flag strings only via review, so the task set is explicit about them.

## Migration Plan

Nothing to deploy: the default is client-side and ships in the binary.
Rollback is reverting the commit. Users who want the old default keep it with
`--format=bar` (commands) or one press of `g` (dashboard) after the upgrade.

## Open Questions

(None.)
