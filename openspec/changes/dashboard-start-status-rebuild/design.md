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

## Decision 3: stamp observations, apply monotonically

`fleet.NodeResult` gains `At time.Time`, set by `FanOutNodes` when the read
returns. The dashboard applies:

> An observation is painted only if its `At` is later than that of the
> observation currently displayed for that node.

This replaces the `fastGen`/`slowGen` counters — it is strictly stronger, since
it also orders a round against an action's completion, which generations cannot.
The case it fixes: a refresh round starts at T0 during a start, the start
finishes at T3 and the tile clears to the node's post-start report, the round
lands at T5 carrying what the node looked like at T0, and today it paints.

`At` also gives the tile an age, which drives Decision 4.

**Alternative rejected:** a per-node sequence number. Same ordering guarantee,
but no age, so the staleness marking would need a second mechanism.

## Decision 4: staleness is visible

A node whose newest observation is older than `3 ×` its kind's interval is drawn
with its age (`· 3m ago`) and drops to the unknown health tier. Three intervals,
not one, so an ordinary late round does not flicker the board grey.

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

## Decision 6: one fold

```go
func dashNodeView(p *StartPhase, obs Observation, now time.Time) (lines []string, tier dashHealthTier)
```

Everything the tile depends on is an argument; nothing is read from a clock or a
field inside. The existing `dashNodeContentLines` switch and
`dashHealthTierFor` switch are merged, because they already have to agree on
priority order and currently do so by duplicated comment rather than by
construction.

The point is the test: a table over `{no phase, each phase} × {no observation,
fresh OK, stale OK, failed} `, which is 20 rows and includes the combination
that produced this bug — and which today has no natural home, because the two
sources meet only inside a `strings.Builder`.

## Risks

- **Signature churn.** `ProgressStarter` is implemented once (`remoteNode`) and
  consumed twice (dashboard, `n.Start`), so the blast radius is small, but the
  CLI's `startProgress` is rewritten as a phase renderer in the same change.
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
