## Why

In `fleet dashboard`, a node with no engine deployed — a daemon reporting
`idle`, or a remote environment asleep at scale-to-zero reporting
`stopped` — shows the same green status dot as a node that is running and
ready. Green reads as "serving", so a grid of undeployed nodes looks
healthy at a glance when it is actually empty. The dot should carry the
node's serving status, not just its absence of failure.

## What Changes

- Add a fifth dashboard health tier for nodes that are up and answering but
  not serving anything: engine state `idle` or `stopped`, with no start or
  stop in flight on the node. Its status dot is grey — the same faded shade
  the `unknown` tier uses, but a filled dot rather than the `?`, so "known
  to be undeployed" stays distinct from "no answer yet".
- `dashHealthTierFor` routes settled `idle`/`stopped` reports to the new
  tier instead of falling through to `healthy`. Everything already ranked
  above it is untouched: an action in flight still reads attention (a node
  that is starting must never read undeployed), a failed outcome still
  reads unhealthy, and a report with no state still reads unknown.
- The health-indicator requirement in the `fleet-client` spec moves from
  four tiers to five, with scenarios for an undeployed node and for
  undeployed-with-start-in-flight.
- `stopped` is covered as well as `idle`: a daemon whose engine was stopped
  and a remote environment that has scaled to zero are the same fact — the
  node is up but serving nothing — and two "off" states wearing two
  different dots would read as inconsistent on the same grid.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-client`: the "Dashboard panels show a health indicator" requirement
  gains a fifth tier — a node whose last refresh reports it not serving
  (`idle` or `stopped`) and has no action in flight reads grey, not green.

## Impact

- `cmd/spinloop/dashboard_render.go` — `dashHealthTier`,
  `dashHealthTierFor`, `dashHealthGlyph`, and their doc comments. No other
  surface draws the glyph: `fleet metrics`, `fleet status`, and the detail
  view are unaffected.
- `cmd/spinloop/fleet_dashboard_test.go` — the byte-stable idle tile and
  the tier table expect the old green for `idle`; both move to the new
  tier, and `stopped` cases join the table.
- No API, config, or dependency changes; no behaviour change outside the
  dashboard tile's status dot.
