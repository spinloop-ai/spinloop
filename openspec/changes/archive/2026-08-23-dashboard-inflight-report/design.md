## Context

The dashboard model already keeps two independent per-node streams: the
refresh rounds, which land their `NodeResult` in the model's `results` on the
group's cadence (2s local, 60s remote) and never look at the actions, and the
per-node action, which carries the verb, the call's latest status line and the
call's cancel. The tile draw — `dashTileContent` in
`cmd/outfit/dashboard_render.go` — was the only layer that knew about both,
and it preferred the action case: whenever a node had an action in flight it
rendered the verb and the action's line and did not consult `results` at all.

So the data the fix wants was already there. See proposal.md for the observed
failure this hid (a client-side start retry reading as a node failure, with
the refresh's state and measurements withheld).

## Goals / Non-Goals

**Goals:**

- The in-flight tile shows the node's last completed refresh — state, serving,
  last-active, resource bars, token counters, whatever the answer carries —
  beside the action's verb and lines.
- The settled tile stays byte-for-byte what it is today (the byte-stable tile
  tests pin it), and the action/refresh lifecycle is untouched.

**Non-Goals:**

- No change to the refresh cadence, the round guards, or the action lifecycle
  (no deadline on a start, abort semantics, one action per node).
- No change to the `remote.Start` retry loop or the client's long-lived start
  request: the "connection dropped" line is the call's own account of itself
  and stays where it is — on the action, not the report.

## Decisions

- **Change the draw, not the model.** The refresh already ran during an
  in-flight action and landed in `results`; only `dashTileContent` withheld
  it. Alternatives: suppressing rounds while an action is in flight (worse —
  the report would go stale exactly when it is wanted), or cancelling the
  action's context when a fresh answer lands (changes the action lifecycle and
  would report a successful slow boot as interrupted). Both rejected.
- **One report body for both tile shapes.** The serving line, last-active
  line and resource/token block are factored into shared helpers used by the
  settled and in-flight cases, so the two cannot word a number differently —
  the same single-sourcing rule the tile already shares with `fleet metrics`.
  The settled case keeps its rule that the resource bars and token counters
  show only for a running node; the in-flight case draws them whenever the
  answer carries them, because a boot half done has some facts and not the
  rest.
- **A failed or unlanded refresh changes nothing on the tile.** The in-flight
  draw falls back to the verb and the call's lines. The action's lines remain
  the account of the call; a transient stats failure is the state of the
  refresh path, not an outcome of the node, and painting it under an in-flight
  start would read as a failure of the node.
- **The state line prints only when non-empty.** Both node kinds' metric
  answers carry a state (the daemon's engine state; the control plane's
  instance state), so it is usually there — but the draw must not invent a
  line for an answer without one.
- **No new geometry.** The tile keeps its 42×12 shape and its hard cut: with
  verb, action line, state and a full bar block in one tile, the token
  counters are the first lines cut, which matches the existing
  truncation-when-tall behaviour rather than adding a second layout.

## Risks / Trade-offs

- [A one-GPU booting node crowds the tile: the token block is cut before the
  bars are shown] → Accepted: the bars are the facts a boot is worth watching,
  and the tile already truncates tall content the same way (four-GPU settled
  node).
- [The state line and the action's verb can both read "starting", from
  different sources (control-plane state vs. the operator's action)] → Kept:
  they are the two truths the change exists to show together; disambiguation
  would decorate one of them and hide the other.
- [The in-flight tile can show an answer a round landed before the action's
  latest line] → Inherent to two independent streams; the tile is a snapshot
  of both at draw time, and each keeps refreshing on its own clock.

## Migration Plan

Display-only change in one CLI view; no persisted state, no API, no
configuration. Nothing to migrate or roll back beyond reverting the commit.
