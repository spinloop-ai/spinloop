## Why

The dashboard maintains two separate descriptions of a node — the status lines a
start writes as it runs, and the last completed refresh round — and no code
compares them for recency or resolves a contradiction between them. The tile
renders both as though each were current.

The reported symptom: a remote start refused for capacity, then granted, left the
tile displaying

```
dev-1  starting
instance no-capacity; retrying in 120s
running  (up 2m 58s)
```

for the remainder of the boot — a capacity wait and a running instance on one
tile, rendered identically. That line was corrected in `03718af` by passing
`remote.Start`'s `onState` callback through to the tile, and an elapsed counter
was added so that a line which stays constant is still visibly updating. The
structure that allowed the defect is unchanged, and it allows more than one:

- **A log line is rendered as current state.** `dashAction.line` is a `string`
  with no timestamp and no expiry. A line written before a wait ("retrying in
  120s") is accurate in a scrolling terminal, where it is one entry in a
  sequence, and inaccurate on a tile that redraws in place, where it is the only
  text shown. Keeping it accurate requires every write site to overwrite it at
  every transition; `remoteNode.StartWithProgress` did not, which produced this
  defect.
- **No reading records when it was taken.** `fleet.NodeResult` carries no
  timestamp, so a 60-second-old `running (up 58s)` is drawn identically to one
  read a moment ago. It also means a refresh round issued before an action
  completes and returned after it will overwrite the post-action report with
  pre-action data: `dashRefreshMsg` checks only a per-group counter, which does
  not order a round against an action completing.
- **The read interval does not vary with what the operator is doing.** A remote
  node stays on its 60-second interval throughout a start — the period in which
  its state changes most and is being watched.
- **No single function produces a tile's contents.** `dashNodeContentLines`
  concatenates the action's lines and the refresh report through a switch over
  several fields. No function takes every input the tile depends on as
  arguments, so the combinations in which the two descriptions contradict each
  other cannot be enumerated in a test.

## What Changes

- **A start reports a phase, not a line.** `fleet.ProgressStarter` carries a
  `StartPhase` value — what the start is doing, when that began, and when the
  next attempt is due — in place of `func(string)`. Each transition replaces the
  value, and nothing accumulates, so a superseded situation is not retained.
  `remoteNode` builds phases from the `progress` and `onState` callbacks
  `remote.Start` already provides.
- **Wait text is computed at draw time.** The rendered text is a pure function of
  the phase and the current time, so "retrying in 120s" counts down and "waiting
  for capacity" counts up, rather than remaining at the value held when the line
  was written.
- **Both screens render the same phases.** `cmd/spinloop`'s `startProgress` (the
  CLI's stderr output) formats the same values, so the dashboard and `spinloop
  remote start` cannot render the same phase differently — the same reason the
  tile and the detail view already share `dashNodeContentLines`.
- **Every reading records when it was taken.** `fleet.NodeResult` gains the time
  of its read, and the dashboard applies one rule throughout: never display a
  reading older than the one currently displayed. This replaces the existing
  counters and corrects the case where a slow round returns late and overwrites
  newer data.
- **An old reading is no longer rendered as current.** A node whose last answer
  is older than a few of its own intervals displays its age and is assigned the
  unknown health tier, rather than presenting an out-of-date state as current.
- **A node with an action in flight is read more often.** It refreshes on the
  short interval until the action completes, then returns to its kind's
  interval. The additional call volume is limited to nodes the operator is
  acting on.
- **One function produces a tile's contents.** It takes the phase, the last
  reading, that reading's age and the current time, and returns the tile's lines
  and its health tier. Every input is an argument, so every combination can be
  enumerated in one test — including the contradictory ones the current code
  cannot test.

## Capabilities

### New Capabilities

(None)

### Modified Capabilities

- `fleet-client`: the "The dashboard drives the selected node" requirement's
  in-flight clause is restated in terms of a start's phase rather than its
  status lines, with the countdown and the elapsed time; "The fleet refreshes
  without stalling" gains the rule that an older reading never replaces a newer
  one, and the shorter interval for a node with an action in flight; "Dashboard
  panels show a health indicator" adds an out-of-date reading as a reason to
  show unknown.

### Unchanged

The keys, the grid, the detail view, the abort behaviour, the confirmation on
stop, and the layout of a tile with no action in flight.

## Impact

- Code: `internal/fleet/node.go` (`ProgressStarter` signature, `StartPhase`,
  `NodeResult.At`), `internal/fleet/remote_node.go` (building phases),
  `internal/fleet/fanout.go` (timestamping results as each read returns),
  `cmd/spinloop/dashboard_model.go` (displaying only newer readings, the shorter
  interval during an action, and the single function producing a tile's
  contents), `cmd/spinloop/dashboard_render.go` (rendering a phase),
  `cmd/spinloop/remote.go` (`startProgress` rendering the same phases).
- `internal/remote` is unchanged: `Start`'s `progress` and `onState` callbacks
  already supply everything the phases are built from, `StateInFlight` included.
- Tests: one test enumerating every phase against every state a reading can be
  in, tests that an older reading never replaces a newer one, and a test that a
  remote node is read more often while an action is in flight.
- Docs: `AGENTS.md`'s dashboard entry, and `docs/internals.md` if the phase
  contract needs a note there.

## Non-goals

- Changing what the control plane reports, or adding states to it.
- Any change to `fleet metrics --watch`, the one-shot commands, or the daemon
  API.
- Cancelling a start's work. An abort still ends only the dashboard's wait, as
  currently specified.
