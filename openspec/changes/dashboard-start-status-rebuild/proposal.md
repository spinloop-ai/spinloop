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
- **Every reading records when it was taken.** `fleet.NodeResult` gains the time
  its read happened, and the dashboard follows one rule everywhere: never show a
  reading older than the one already on screen. This replaces the existing
  generation counters and fixes the case where a slow round lands late and
  repaints old data over new.
- **An old reading stops being drawn as if it were current.** A node whose last
  answer is older than a few of its own intervals shows its age and drops to the
  unknown health tier, instead of presenting an out-of-date state as fact.
- **A node being acted on is read more often.** A node with an action in flight
  refreshes on the short interval until it settles, then returns to its kind's
  cadence. The extra cost is limited to the nodes the operator is touching.
- **One function decides what a tile says.** It takes the phase, the last
  reading, how old that reading is and the current time, and returns the tile's
  lines and its health colour. Everything the tile depends on is an argument, so
  every combination can be listed in one test — including the contradictory ones
  that have nowhere to be tested today.

## Capabilities

### New Capabilities

(None)

### Modified Capabilities

- `fleet-client`: the "The dashboard drives the selected node" requirement's
  in-flight clause is restated in terms of a start's phase rather than its
  status lines, with the countdown and the elapsed time; "The fleet refreshes
  without stalling" gains the rule that an older reading never replaces a newer
  one, and the faster cadence for a node being acted on; "Dashboard panels show
  a health indicator" adds an out-of-date reading as a reason to show unknown.

### Unchanged

The keys, the grid, the detail view, the abort semantics, the confirmation on
stop, and the settled (no action in flight) tile's layout are all untouched.

## Impact

- Code: `internal/fleet/node.go` (`ProgressStarter` signature, `StartPhase`,
  `NodeResult.At`), `internal/fleet/remote_node.go` (phase construction),
  `internal/fleet/fanout.go` (stamping results as each read returns),
  `cmd/spinloop/dashboard_model.go` (only showing newer readings, the faster
  cadence during an action, and the one function that decides a tile's
  contents), `cmd/spinloop/dashboard_render.go` (drawing a phase),
  `cmd/spinloop/remote.go` (`startProgress` drawing the same phases).
- `internal/remote` is untouched: `Start`'s `progress`/`onState` pair already
  carries what the phases are built from, including `StateInFlight`.
- Tests: one test listing every phase against every state a reading can be in,
  tests that an older reading never replaces a newer one, and a test that a
  remote node is read more often while an action is in flight.
- Docs: `AGENTS.md`'s dashboard pointer and `docs/internals.md` if the phase
  contract warrants a note there.

## Non-goals

- Changing what the control plane reports, or adding states to it.
- Any change to `fleet metrics --watch`, the one-shot surfaces, or the daemon
  API.
- Making a start's work abortable. An abort still ends the dashboard's wait
  only, exactly as specified today.
