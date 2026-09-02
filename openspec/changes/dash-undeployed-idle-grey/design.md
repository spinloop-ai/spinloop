## Context

`fleet dashboard` renders each node as a fixed tile whose name line is
prefixed by a status dot: `dashHealthGlyph(dashHealthTierFor(result, action))`
in `cmd/spinloop/dashboard_render.go`. Today the tier switch has four
outcomes — attention (action in flight), unknown (no refresh yet, or a
report with no state), unhealthy (failed outcome or `crashed`), and
everything else — and `idle`/`stopped` reports fall into "everything else",
drawing the green dot of a serving node. See proposal.md for why that is
wrong. The tile body already prints the state word ("idle", "stopped") on
its state line, so the text is never in doubt; only the dot lies.

Constraints: the glyph is raw ANSI in the same style as the resource bars
(the tile body is one plain string under a single lipgloss border style),
and the tile's content lines must stay byte-identical to what the `fleet
metrics` bar format prints for the node — the dot is tile-only, prepended
in `dashTileContent`, and is the one thing the byte-stable tile tests pin
beside the shared lines.

## Goals / Non-Goals

**Goals:**
- A settled report of `idle` or `stopped` with no action in flight draws a
  faded grey dot instead of green.
- The new grey stays distinguishable from the `unknown` grey, which marks
  "no answer yet" with a `?`.
- Every precedence rule the tier switch already enforces (action in flight
  beats everything; failed outcome beats state; empty state reads unknown)
  is preserved unchanged.

**Non-Goals:**
- No change to any tile body line, to `fleet metrics`/`fleet status`
  output, or to the detail view (none of them draw the dot).
- No new state vocabulary: `idle`/`stopped` are exactly the daemon states
  and the remote control-plane state that mean "not serving", so the tier
  keys on the state strings it already sees.
- No colour for states like a remote environment reporting `starting`
  outside of a dashboard-initiated start — that path is covered by the
  existing action-in-flight rule when the dashboard drove the start, and
  untouched otherwise.

## Decisions

**1. A new tier, not a reuse of `dashUnknown`.**
Add `dashNotServing` to the `dashHealthTier` enum with its own case in
`dashHealthGlyph`. Reusing `dashUnknown` would render an undeployed node
with the `?` mark, erasing the difference between "the dashboard hasn't
heard from this node" and "it has, and the node is up with nothing
deployed" — the exact distinction the spec's "different marks" scenario
locks in.

**2. The glyph is `\033[90m●\033[0m`: the existing faded shade, the dot
kept.**
SGR 90 (bright black) is the file's established "faded" code —
`dashUnknown` already uses it for its `?` — so the new tier introduces no
new colour and works on 16-colour terminals like the rest of the palette
(92/33/31/90). The mark tells the two greys apart: filled dot = known
state, `?` = no answer. Alternative considered: a darker 256-colour grey
matching the unselected border (`240`) — rejected, it needs the 256-colour
profile and would make the tile the only part of the view that breaks on a
16-colour terminal.

**3. The new case sits in `dashHealthTierFor` after the empty-state case,
immediately before `default`.**
The switch's order is its precedence. The action-in-flight, no-outcome,
failed-outcome/`crashed`, not-ready, and empty-state cases all come first
and are untouched, so a start in flight over an `idle` report still reads
attention and a failed refresh still reads unhealthy regardless of state.
`idle` and `stopped` are non-empty states, so the new case overlaps
nothing; placing it just before `default` is the only position that
changes any behaviour, and it changes exactly the two states named in the
spec.

**4. `stopped` is in scope alongside `idle`.**
A daemon that stops its engine reports `stopped`; a remote environment at
scale-to-zero reports `stopped` (the control-plane state passes through
`statusFromRemote` unchanged). Both are "node up, serving nothing" — the
same fact the dot exists to convey — and keying the tier on `idle` alone
would leave two off states wearing two different dots on one grid. The
proposal records this as a scoping decision.

**5. Tile body untouched; tests move with the glyph.**
Only `dashHealthTierFor` and `dashHealthGlyph` change, so the
content-line invariant (tile lines = `fleet metrics` bar lines) is intact
by construction. In `fleet_dashboard_test.go`, the byte-stable idle tile
(`TestDashTileStoppedByteStable`) and the tier table's `idle is healthy`
entry change their expected glyph to the new tier, and the table gains
`stopped` and idle-with-start-in-flight rows.

## Risks / Trade-offs

- [The byte-stable tile tests pin the glyph bytes, so a wrong escape
  sequence ships as a test failure, not a surprise] → they are the check;
  the ASCII-profile tests render the new dot as plain `●` like the others.
- [`unknown` and `not serving` now share a colour] → the spec requires
  different marks (`?` vs filled dot), and the tile's state line repeats
  the word, so a misread needs both the mark and the text to be missed.
- [A remote control plane reporting an off-state string not covered
  (e.g. a future state name) would fall through to green] → same
  limitation the current switch already has for any unexpected state; the
  tier is deliberately keyed on the states the daemon and control plane
  actually report today.

## Migration Plan

No migration: a visual change to one dot in one view, no data, no API, no
config. Rollback is reverting the change; nothing written to disk is
affected.
