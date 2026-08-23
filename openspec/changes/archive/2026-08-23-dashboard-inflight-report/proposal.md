## Why

While a cloud node's start is in flight, the dashboard's tile shows only the
start call's own progress lines and hides the node's refresh report. A boot
that outlives the client's long-lived start request reports
"connection dropped (context deadline exceeded); retrying in 5s" on every
client-side retry, and with the refresh report withheld the tile reads as a
node that has failed — even though the refresh rounds keep landing the
instance's state and, minutes into a boot, its resource usage, and the
operator has no way to see either.

## What Changes

- The in-flight dashboard tile (a start or stop under way on the node) now
  shows the node's last completed refresh report **beside** the action's verb
  and status lines, instead of in place of it: the node's state, what it
  serves, its last-active record, and its resource bars and token counters —
  whatever of those the report carries. The call's lines say what the
  operator asked for; the report says what the node is doing.
- The fallbacks are unchanged: no round has landed yet, or the latest round
  failed — the tile shows the verb and the call's lines alone, as before.
- A finished action still clears the tile to the node's report and leaves its
  outcome on the status line; the settled (no action in flight) tile is
  byte-for-byte unchanged, including its rule that the resource bars and
  token counters show only for a running node.
- The resource bars and token counters on the in-flight tile are drawn
  whenever the report carries them, without the running-state gate: a boot
  half done has some of the facts and not the rest, and whatever it has is
  what the tile should show while the start still works.

## Capabilities

### New Capabilities

(None)

### Modified Capabilities

- `fleet-client`: the "The dashboard drives the selected node" requirement's
  in-flight clause changes — while an action is in flight, the node's tile
  carries the verb and the action's status lines beside the node's last
  report (the report's state, serving and whatever it measures), rather than
  in place of it; the "A start is watched on its own tile" scenario gains the
  report showing alongside.

## Impact

- Code: `cmd/outfit/dashboard_render.go` — the in-flight case of
  `dashTileContent`, plus two shared helpers for the report body (serving
  line, last-active line, resource bars and token counters) used by both the
  settled and in-flight tiles so the two cannot word a number differently.
  No model, refresh or action lifecycle changes: the refresh rounds already
  ran and landed during an in-flight action; only the tile's drawing changes.
- Tests: `cmd/outfit/fleet_dashboard_test.go` — byte-stable in-flight tile
  cases with a landed report (measuring, early boot, failed round) and a
  model-level case of a round landing beside an in-flight action.
- Docs: the in-flight-tile clause in AGENTS.md and the renderer/action doc
  comments.
