## Why

The dashboard has no single account of what a node is doing. It draws two
independent things side by side — the status lines a start reports as it works,
and the last completed refresh round — and has no rule for which is fresher or
what to do when they contradict each other. The result is a board that states
things confidently and wrongly.

The reported symptom: a remote start refused for capacity, then granted, left
the tile showing

```
dev-1  starting
instance no-capacity; retrying in 120s
running  (up 2m 58s)
```

for the rest of the boot — a capacity wait and a running instance, on one tile,
presented as equally current. That specific line was fixed in `03718af` by
wiring `remote.Start`'s `onState` through to the tile, and an elapsed counter
was added so a line that stands still is visibly live. The shape that produced
the bug is still in place, and it produces more than one:

- **A transcript is rendered as a status.** `dashAction.line` is a `string`
  with no timestamp and no notion of being superseded. A line written before a
  wait ("retrying in 120s") is true in a scrolling terminal, where it is a
  historical entry, and false on a tile that never scrolls, where it is a claim
  about the present. Every producer of those lines has to remember to overwrite
  them; the one that forgot caused this bug.
- **Nothing records when it was read.** `fleet.NodeResult` carries no time, so a
  60-second-old `running (up 58s)` is drawn identically to one read a moment
  ago. It also means a refresh round that *started* before an action finished
  and *landed* after it repaints pre-action data over the result of the action:
  `dashRefreshMsg` only checks a per-group generation counter, which does not
  catch that ordering.
- **Cadence ignores attention.** A remote node stays on its 60-second interval
  throughout a start — the one window in which its state is changing and being
  watched.
- **No single place decides what a tile says.** `dashNodeContentLines`
  concatenates the action narrative and the refresh report through an ad-hoc
  switch. There is no function whose inputs are everything the tile depends on,
  so the contradictory combinations — the ones that produce bugs — have nowhere
  to be tested.

## What Changes

- **A start reports a phase, not a line.** `fleet.ProgressStarter` carries a
  `StartPhase` value — what the start is doing, when that began, and when the
  next attempt is due — instead of `func(string)`. Each transition replaces the
  phase; nothing accumulates, so there is no store for a superseded line to sit
  in. `remoteNode` builds phases from the `progress`/`onState` pair
  `remote.Start` already offers.
- **Wait text becomes a live countdown.** Rendering a phase is a pure function
  of the phase and the current time, so "retrying in 120s" counts down and
  "waiting for capacity" counts up, rather than freezing at whatever the number
  was when the line was written.
- **One renderer for both surfaces.** `cmd/spinloop`'s `startProgress` (the CLI's
  stderr heartbeat) renders the same phase stream, so the dashboard and
  `spinloop remote start` cannot word the same situation differently — the same
  reason the tile and the detail view already share `dashNodeContentLines`.
- **Every observation is stamped.** `fleet.NodeResult` gains the time its read
  was taken, and the dashboard applies one rule everywhere: never paint an
  observation older than the one already displayed. This subsumes the existing
  generation counters and fixes the late-round-repaints-stale-data case.
- **A stale reading stops being drawn as fact.** A node whose last answer is
  older than a few of its own intervals is marked as such and drops to the
  unknown health tier, rather than showing an aged state confidently.
- **Attention drives cadence.** A node with an action in flight refreshes on the
  fast interval until it settles, then returns to its kind's cadence. Bounded
  cost: only nodes the operator is currently acting on.
- **One fold, table-tested.** A single function takes the phase, the last
  observation, its age and the current time, and returns the tile's lines and
  its health tier. Every combination becomes a row in a table test, including
  the contradictory ones that have no coverage today.

## Capabilities

### New Capabilities

(None)

### Modified Capabilities

- `fleet-client`: the "The dashboard drives the selected node" requirement's
  in-flight clause is restated in terms of a start's phase rather than its
  status lines, with the countdown and the elapsed time; "The fleet refreshes
  without stalling" gains the monotonic-observation rule and the attention
  cadence; "Dashboard panels show a health indicator" gains staleness as a
  route to the unknown tier.

### Unchanged

The keys, the grid, the detail view, the abort semantics, the confirmation on
stop, and the settled (no action in flight) tile's layout are all untouched.

## Impact

- Code: `internal/fleet/node.go` (`ProgressStarter` signature, `StartPhase`,
  `NodeResult.At`), `internal/fleet/remote_node.go` (phase construction),
  `internal/fleet/fanout.go` (stamping results as each read returns),
  `cmd/spinloop/dashboard_model.go` (monotonic apply, attention cadence, the
  fold), `cmd/spinloop/dashboard_render.go` (phase rendering),
  `cmd/spinloop/remote.go` (`startProgress` as a phase renderer).
- `internal/remote` is untouched: `Start`'s `progress`/`onState` pair already
  carries what the phases are built from, including `StateInFlight`.
- Tests: a table test over the fold covering every phase against every
  observation state, ordering tests for the monotonic rule, and a cadence test
  for a remote node with an action in flight.
- Docs: `AGENTS.md`'s dashboard pointer and `docs/internals.md` if the phase
  contract warrants a note there.

## Non-goals

- Changing what the control plane reports, or adding states to it.
- Any change to `fleet metrics --watch`, the one-shot surfaces, or the daemon
  API.
- Making a start's work abortable. An abort still ends the dashboard's wait
  only, exactly as specified today.
