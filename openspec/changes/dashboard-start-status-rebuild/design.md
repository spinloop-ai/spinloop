## Context

Three things report on a node, on three different clocks:

| source | cadence | today's shape |
| --- | --- | --- |
| the start call's own progress | irregular — only before a wait | `string`, last one wins, kept forever |
| the refresh round | 2s local, 60s remote | `fleet.NodeResult`, no timestamp |
| the operator's action | on keypress | `dashAction{verb, line, cancel, aborted, since}` |

The tile concatenates the first and second. Nothing reconciles them, and only
the second has any freshness control at all (per-group generation counters, which
order rounds against each other but not against anything else).

## Decision 1: a start reports a phase, not a line

```go
// StartPhaseKind is what a start is doing right now. Exactly one holds at a
// time, and a new one replaces its predecessor outright.
type StartPhaseKind int

const (
    PhaseAttempting   StartPhaseKind = iota // a request is out, no reply yet
    PhaseWaitingCapacity                    // refused for want of capacity, waiting to retry
    PhaseBooting                            // accepted; the instance is coming up
    PhaseReconnecting                       // the request's connection dropped, waiting to retry
)

// StartPhase is a start's whole situation, as a value. It carries no prose:
// the caller words it, so the dashboard and the CLI cannot disagree about what
// the same phase means.
type StartPhase struct {
    Kind    StartPhaseKind
    Since   time.Time // when this phase began
    RetryAt time.Time // when the next attempt is due; zero unless waiting
    Detail  string    // the control plane's own state, or a transport error
}
```

`ProgressStarter` becomes:

```go
type ProgressStarter interface {
    StartWithProgress(ctx context.Context, report func(StartPhase)) (daemon.StatusResponse, error)
}
```

Why a value and not a better-behaved string: the bug was not bad wording, it was
that a `string` has nowhere to record *when it stopped being true*. A phase makes
supersession structural — there is one field, and writing it retires whatever was
there. A producer cannot forget to overwrite, because there is no second slot.

`remoteNode` maps `remote.Start`'s existing callbacks onto phases:
`onState(StateInFlight)` → `PhaseAttempting`; `onState("no-capacity")` +
the 503's `RetryAfterSeconds` → `PhaseWaitingCapacity` with `RetryAt`;
`onState` of any other state on a held request → `PhaseBooting`; the
`connection dropped` progress line → `PhaseReconnecting`. `internal/remote`
does not change.

**Alternative rejected:** keep `func(string)` and require every producer to
re-emit on supersession. That is what the quick fix did, and it works only for as
long as every future producer remembers. The failure mode is silent and looks
exactly like correct output.

## Decision 2: rendering is a function of (phase, now)

```go
func RenderPhase(p StartPhase, now time.Time) string
```

`PhaseWaitingCapacity` renders `waiting for capacity — retrying in 47s`,
recomputed each repaint from `RetryAt.Sub(now)`. `PhaseBooting` renders
`booting (4m 12s)` from `now.Sub(p.Since)`. The board already repaints on its
2-second tick, so no new timer is needed.

This is what makes a frozen number impossible rather than merely unlikely: there
is no stored number to freeze. The elapsed time added by the quick fix is the
same idea applied to the verb, and it collapses into this.

## Decision 3: record when each reading was taken, and never go backwards

`fleet.NodeResult` gains `At time.Time`, set by `FanOutNodes` when the read
returns. The dashboard then follows one rule:

> Show a reading only if it was taken later than the one currently on screen for
> that node.

This replaces the `fastGen`/`slowGen` counters, and covers more than they can:
the counters put rounds in order against each other, but cannot put a round in
order against an action finishing. The case that fixes: a refresh round starts
at T0 during a start, the start finishes at T3 and the tile clears to the node's
report from after the start, the round lands at T5 carrying what the node looked
like at T0 — and today the tile paints it.

`At` also tells the tile how old its reading is, which Decision 4 needs.

**Alternative rejected:** a counter per node. It orders readings just as well,
but says nothing about age, so showing how old a reading is would need a second
mechanism alongside it.

## Decision 4: an old reading says so

A node whose newest reading is older than three times its kind's interval is
drawn with its age (`· 3m ago`) and drops to the unknown health tier. Three
intervals rather than one, so an ordinary late round does not flicker the board
grey.

This is the general form of the reported bug: the board's failure was not that it
showed old data, it was that it showed old data *indistinguishably from fresh
data*. Once the age is on the tile, an operator can see the difference even in a
case nobody anticipated.

## Decision 5: attention drives cadence

A node with an action in flight refreshes on `dashboardRefreshInterval`
regardless of kind, returning to its kind's cadence once the action settles.

A remote node is on 60 seconds because a signed Lambda call is expensive and its
state changes slowly — both true of an idle environment, neither true of one the
operator just pressed start on. The cost is bounded by the number of nodes with
an action in flight, which is bounded by the operator's hands.

## Decision 6: one function decides what a tile says

```go
func dashNodeView(p *StartPhase, reading Reading, now time.Time) (lines []string, tier dashHealthTier)
```

Everything the tile depends on is an argument. Nothing is read from a clock or
from a field on the model, so the same arguments always produce the same tile.
This merges the existing `dashNodeContentLines` and `dashHealthTierFor`
switches, which already have to agree on the order they check things in and
currently manage it by saying so in a comment on each.

The reason to do it is the test. With one function there is a single place to
list every phase against every state a reading can be in — no reading yet, a
fresh answer, a stale answer, a failed round — which is twenty cases, and
includes a start waiting for capacity next to a reading that says the node is
running. That is the case that caused this bug, and today it has nowhere to be
tested, because the two accounts of the node only meet inside a
`strings.Builder`.

## Risks

- **The interface changes.** `ProgressStarter` is implemented once
  (`remoteNode`) and called from two places (the dashboard, and `n.Start`), so
  not much has to change, but the CLI's `startProgress` is rewritten in the same
  change to draw the same phases.
- **Behavioural drift in the CLI.** `spinloop remote start`'s stderr lines change
  wording. They are progress output, not parsed by anything (the eval-able
  `export` lines go to stdout and are untouched), but the change is user-visible
  and the CLI tests pin some of it.
- **The staleness threshold is a guess.** Three intervals is chosen to avoid
  flicker; if a slow cloud round routinely crosses it the board will grey out
  more than it should, and the multiplier is the dial.

## Migration

No config, no on-disk state, no API surface. The change is entirely internal plus
the wording on two screens.
