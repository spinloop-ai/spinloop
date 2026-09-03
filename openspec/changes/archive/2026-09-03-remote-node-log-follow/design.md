## Context

`internal/fleet.Node.Logs(ctx, offset, limit)` is the one method every node
kind — local daemon and remote environment — answers to serve `fleet logs`
and the dashboard's detail view. A local daemon node's `offset` is a real
byte position in a file, so the interface's int64 cursor maps onto it
directly. A remote environment's log store (CloudWatch, via
`internal/remote.FetchLogs`) has no equivalent resumable position: a query
only takes a `Start` time bound, and the shipping agent's delivery lag means
a bound alone can both miss events (bound set past them) and repeat events
(bound set before them, re-read on the next poll) — which is exactly why
`spinloop remote logs -f` already carries its own event-id dedup, keyed off
an overlap window behind the newest event seen, rather than using `Start`
as an exact cursor. See proposal.md for why the remote node's `Logs` skipped
all of this and what broke as a result.

## Goals / Non-Goals

**Goals:**
- One implementation of the dedup-by-id/overlap-window follow logic, used by
  both `spinloop remote logs -f` and a remote fleet node's `Logs` call, so a
  future change to that logic cannot fix one and leave the other broken.
- Keep `internal/fleet.Node`'s interface unchanged — the fix lives entirely
  inside `remoteNode`.

**Non-Goals:**
- Making a remote node's `Logs` cursor exact (immune to the rare
  same-millisecond edge case the overlap window already tolerates for the
  standalone command). Matching the standalone command's existing behavior
  is the bar, not exceeding it.
- Changing anything about a local daemon node's log reading; it already
  meets the no-duplicate requirement via a real byte offset.

## Decisions

**Extract the standalone follow's dedup state into `internal/remote.FollowCursor`,
and have `remoteNode` hold one per node instance.**

`cmd/spinloop/remote_logs.go`'s `followLogsLoop` already carried exactly the
state needed (`printed map[string]time.Time`, `newest time.Time`) inline as
loop-local variables. Rather than reimplement similar logic against
`remoteNode`'s different lifetime (one `Logs` call per poll instead of one
loop), that state is lifted into a small exported type in `internal/remote`
— the package both call sites already depend on — with `Advance` (filter
events to the unseen ones, fold them into the cursor) and `Start` (the next
query's lower bound) as its interface. `followLogsLoop` now calls the same
two methods instead of its inline map.

`remoteNode` holds its own `*FollowCursor` because the `Node.Logs(offset,
limit)` interface has nowhere else to keep it: the caller (`fleet.LogsCall`,
the dashboard's detail view) only threads back a single `int64`, which
cannot carry an id set. `remoteNode` is already a long-lived object — built
once per fleet session and called repeatedly across polls — so it is the
natural place for this state to live, the same way one `followLogsLoop`
invocation is the natural place for the standalone command's state to live.

The `offset int64` the interface still passes is repurposed as a binary
signal rather than a position: `daemon.TailLog` means "start the cursor
over" (a fresh open of the view should show its own tail, not have it
suppressed as already seen by a previous open), anything else means
"continue". The real position lives in the node's own cursor.

**Alternative considered**: encode the cursor as a plain millisecond
timestamp round-tripped through `offset`/`NextOffset`, bounding each query at
`offset+1ms` with no id-based dedup. This was the first fix attempted; it
removes the constant full-tail replay but reintroduces exactly the
same-millisecond duplicate/loss edge case the standalone command's overlap
window already solves, via a second, slightly different mechanism. Rejected
in favor of sharing the proven mechanism outright, which is also what the
user asked for directly.

## Risks / Trade-offs

- [The overlap window can still miss an event that shares its exact
  millisecond with the event that set the window's edge, on both the
  standalone command and the fleet node.] → Unchanged pre-existing behavior
  of the mechanism being shared; not worsened by this change, and out of
  scope per Non-Goals.
- [`remoteNode.Logs` fanning out over more than one engine log group
  (`llamacpp`, `vllm`) can deliver the same event twice in one raw response
  before dedup runs, if both groups happen to carry it.] → `FollowCursor.Advance`
  dedupes by event id across the whole batch it is given, not per group, so
  this collapses to one before any content is returned.
