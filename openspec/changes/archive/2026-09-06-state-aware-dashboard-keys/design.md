# Design: state-aware dashboard key hints

## Context

See proposal.md for the motivation (issue #173). The state of the code the
change lands in:

- The dashboard's key help is one footer line, drawn by `footerLine` for both
  screens. The grid's line comes from `dashModel.gridKeys`, the detail view's
  from `dashModel.detailKeys`; each is a fixed string that already drops the
  keep entry where `keepOffered` says it would drive nothing, and the abort
  entry is dropped after the fact by `dashFooterHints` where `canAbort` says
  nothing is abortable.
- The node the line describes is one: the entry under the cursor on the grid,
  the entry in view in the detail view (the cursor does not move while the
  view is open).
- The board holds one read per node (`results`, parallel to `entries`), and
  the tile renders exactly that read — an answered report with a state, or a
  failure with no state. A read answers "running" when it is OK and its
  metrics carry a running state.
- The keys themselves already guard what they do: `s` and `x` drive nothing on
  a node with an action in flight, `beginAction` refuses a nil node, and the
  node operations behind them refuse start on a running engine and stop on a
  non-running one. The hints simply do not follow those guards yet.

## Goals / Non-Goals

**Goals:**

- The key help names `s` and `x` only where the key it names would drive
  something on the node the line describes, on both screens, with the same
  gate the key handler answers to.
- The existing entry order and wording of each footer line is preserved; only
  entries appear and disappear.

**Non-Goals:**

- Changing what `s`, `x`, `k`, or `a` do, or when the key handlers refuse
  them: the keys keep their current guards, and the status line keeps saying
  so when a key is pressed anyway.
- Showing per-tile key hints inside the tiles: the footer is the one key help
  line, and it is already about the selected node.
- Gating the keys that do not depend on the node's state (move, format,
  refresh, quit, back, follow, and the keep prompt's own keys).

## Decisions

### 1. Gate on the board's current read, not a separate record of the last
answered one

Each offer predicate reads the same `fleet.NodeResult` the tile renders for
the described node: "running" means that result is OK and reports a running
state. A failed read, and the empty result before the first answered one,
report no state, which is not running — so the line offers start, not stop,
exactly while the tile shows the failure or "waiting for first refresh".

Keeping one source of truth for what the node is doing is what lets the tile
and the footer never disagree: the footer is read from the same value the
panel was drawn from, with no second store to drift out of step with it.

Alternative considered: remembering the last answered read per node so a
transient failed read would keep offering stop for a node last seen running.
That stores a second account of the node's state beside the one the tile
draws from, and the tile itself shows no state while the read fails — the
footer would then advertise a stop the panel does not show a reason for.

### 2. The offers live on the model, one predicate per key

`gridKeys` and `detailKeys` each build their line from the per-key offers for
the described node — a `startOffered` and a `stopOffered` beside the existing
`keepOffered` and `canAbort` — joined in each screen's existing entry order.
The predicates take no arguments beyond the model: the described node is
`entries[cursor]` for both screens, the read is `results[cursor]`, and the
action is `actions[cursor]`.

- start is offered when the node exists, has no action in flight, and the
  board's current read of it does not report it running.
- stop is offered when the node exists, has no action in flight, and the
  board's current read of it reports it running.

The node-existence check is the one `keepOffered` already gets for free
(its capability assertion fails on a nil node): a node that could not become
a node offers no start and no stop either, matching `beginAction`'s own
refusal for it.

Alternative considered: keeping the fixed strings and filtering entries out of
the rendered line, as the abort entry is done today. That is the string
surgery the code is moving away from — the keep change already gates at
construction — and a fourth filter for a second key would make the footer's
contents a function of which substrings survived, rather than of which keys
are offered.

### 3. The abort filter is folded into the construction

With the abort entry one of the gated parts, `dashFooterHints` — the
string filter that exists only to drop the abort entry — has no work left and
is removed; the grid's `View` passes the constructed line to `footerLine`
directly. The abort predicate is unchanged, so the abort's existing behaviour
and spec wording are untouched.

### 4. The keys are not gated

Only the hints change. `s` on a running node and `x` on a stopped one still
drive nothing and leave the node's own reason on the status line, so an
operator who knows the board better than its hints, or a key pressed a beat
ahead of the read that would hide it, still gets the answer the one-shot
surface gives.

## Risks / Trade-offs

- [A failed read following a running read offers start rather than stop for
  the duration of the failure] → the tile shows no state at the same time, so
  the footer agrees with the panel; pressing start on a node that in fact is
  running fails with the daemon's own reason on the status line, which is the
  same answer the one-shot `fleet start` gives.
- [The line changes length as the selection moves and states refresh] → that
  is the existing behaviour for the keep and abort entries; the line is
  re-cut to the width on every draw, and a shorter line is never a problem
  for a footer.
- [A stop offered on a read that is a beat old] → the offer follows the same
  read the tile draws from, so the footer never claims a state the panel does
  not show; the confirmation the stop still asks for is the guard against an
  operator acting on a moment-stale glance.
