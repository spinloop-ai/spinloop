## Context

Three sources describe a node, updated on three different schedules:

| source | update rate | current representation |
| --- | --- | --- |
| the start call's progress | irregular — written only before a wait | `string`; each write replaces the previous, and the last one is retained indefinitely |
| the refresh round | 2s local, 60s remote | `fleet.NodeResult`, no timestamp |
| the operator's action | on keypress | `dashAction{verb, line, cancel, aborted, since}` |

The tile concatenates the first two. No code compares them, and only the second
has any ordering control: per-group counters that order refresh rounds against
each other and against nothing else.

## Decision 1: a start reports a phase, not a line

```go
// StartPhaseKind identifies what a start is currently doing. Exactly one value
// applies at a time, and each new value replaces the previous one.
type StartPhaseKind int

const (
    PhaseAttempting      StartPhaseKind = iota // a request has been sent, no reply yet
    PhaseWaitingCapacity                       // refused for want of capacity, waiting to retry
    PhaseBooting                               // accepted; the instance is starting
    PhaseReconnecting                          // the request's connection dropped, waiting to retry
)

// StartPhase holds a start's current situation as data rather than text. The
// display layer formats it, so the dashboard and the CLI render the same phase
// identically.
type StartPhase struct {
    Kind    StartPhaseKind
    Since   time.Time // when this phase began
    RetryAt time.Time // when the next attempt is due; zero except while waiting
    Detail  string    // the control plane's state string, or a transport error
}
```

`ProgressStarter` becomes:

```go
type ProgressStarter interface {
    StartWithProgress(ctx context.Context, report func(StartPhase)) (daemon.StatusResponse, error)
}
```

A value rather than a more carefully written string, because the defect was not
the wording of the line: a `string` field records no expiry. A phase holds one
value per start and each write replaces the previous one, so a superseded
situation is not retained. Achieving the same result with a string requires
every write site to overwrite at every transition.

`remoteNode` maps `remote.Start`'s existing callbacks onto phases:
`onState(StateInFlight)` → `PhaseAttempting`; `onState("no-capacity")` plus the
503's `RetryAfterSeconds` → `PhaseWaitingCapacity` with `RetryAt` set;
`onState` with any other state on a held request → `PhaseBooting`; the
`connection dropped` progress line → `PhaseReconnecting`. `internal/remote` is
unchanged.

**Alternative rejected:** keep `func(string)` and write a line at every
transition. That is what the quick fix does. It is correct only while every
write site covers every transition, and an omission produces no error and no
test failure — the output is a well-formed line carrying an out-of-date value.

## Decision 2: the rendered text is a function of the phase and the current time

```go
func RenderPhase(p StartPhase, now time.Time) string
```

`PhaseWaitingCapacity` renders `waiting for capacity — retrying in 47s`,
recomputed on each repaint from `RetryAt.Sub(now)`. `PhaseBooting` renders
`booting (4m 12s)` from `now.Sub(p.Since)`. The board already repaints on its
2-second tick, so this needs no additional timer.

No number is stored, so no number can be left at a stale value. The elapsed
counter added by the quick fix applies the same approach to the verb, and is
replaced by this.

## Decision 3: record when each reading was taken, and never go backwards

`fleet.NodeResult` gains `At time.Time`, set in `FanOutNodes` as each read
returns. The dashboard then applies one rule:

> Display a reading only if it was taken later than the one currently displayed
> for that node.

This replaces the `fastGen`/`slowGen` counters and covers a case they cannot:
the counters order refresh rounds relative to each other, but not relative to an
action completing. Concretely — a refresh round is issued at T0 during a start,
the start completes at T3 and the tile is cleared to the node's post-start
report, the round returns at T5 carrying the node's state as of T0, and the
current code displays it.

`At` also supplies the reading's age, which Decision 4 requires.

**Alternative rejected:** a per-node counter. It orders readings equally well,
but carries no age, so displaying a reading's age would require a second field
alongside it.

## Decision 4: an old reading is displayed with its age

A node whose newest reading is older than three times its kind's interval is
drawn with its age (`· 3m ago`) and assigned the unknown health tier. Three
intervals rather than one, so an ordinary late round does not flicker the board
grey.

This generalises the reported defect. The problem was not that the board
displayed an old reading; it was that an old reading was rendered identically to
a current one. With the age displayed, an operator can distinguish the two in
cases not anticipated here.

## Decision 5: read a node more often while an action is in flight

A node with an action in flight is read on `dashboardRefreshInterval` regardless
of kind, and returns to its kind's interval once the action completes.

A remote node is read once a minute because each read is a signed Lambda call
and an idle environment's state changes slowly. Neither applies to an
environment the operator has just started. The additional call volume is
proportional to the number of nodes with an action in flight.

## Decision 6: one function produces a tile's contents

```go
func dashNodeView(p *StartPhase, reading Reading, now time.Time) (lines []string, tier dashHealthTier)
```

Every input the tile depends on is an argument. No clock is read and no field is
taken from the model, so the same arguments always produce the same output. This
merges the existing `dashNodeContentLines` and `dashHealthTierFor` switches,
which must already evaluate their conditions in the same order and currently
depend on a comment in each to keep them aligned.

The reason to merge them is testability. One function gives a single place to
enumerate every phase against every state a reading can be in — no reading yet,
a fresh answer, a stale answer, a failed round — which is twenty cases, and
includes `PhaseWaitingCapacity` alongside a reading reporting the node running.
That is the combination that produced this defect, and the current code has no
place to test it, because the two descriptions of the node are combined only
inside a `strings.Builder`.

## Risks

- **The interface changes.** `ProgressStarter` has one implementation
  (`remoteNode`) and two call sites (the dashboard, and `n.Start`), so the
  change is small, but the CLI's `startProgress` is rewritten in the same change
  to render the same phases.
- **The CLI's output changes.** `spinloop remote start`'s stderr lines are
  reworded. Nothing parses them (the eval-able `export` lines go to stdout and
  are unchanged), but the change is visible to users and some CLI tests assert
  on them.
- **The staleness threshold is a guess.** Three intervals is chosen to avoid
  flicker. If slow cloud rounds routinely exceed it, the board will grey out
  more often than intended, and the multiplier is the value to adjust.

## Migration

No configuration, no on-disk state, no API changes. The change is internal, plus
the text rendered on two screens.
