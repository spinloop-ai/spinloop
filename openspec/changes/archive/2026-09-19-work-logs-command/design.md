## Context

`cmd/spinloop/work.go` already has `workTarget` (resolves `--url` and the
token) and `workRequest` (one call to the work list API, a non-2xx turned
into an error carrying the API's own message) — every other `work`
subcommand is built on these two. The API's `GET /v1/items/{id}/log`
(`internal/orchestrator/api.go`'s `handleLog`) already answers `{"id":
"<id>", "log": "<content>"}` or `{"id": "<id>", "log": null}`, and refuses
an unknown id the way `remove`/`abort` already do (`WorkList.Log`, an
`errMissing` the API turns into a 404 `workAPIError` already reads).
`GET /v1/items` already answers each item's `state` (`ItemView`, the same
type `work list` already decodes).

`cmd/spinloop/follow.go`'s `followUntilInterrupted` is the shared
poll-under-a-cancellable-context loop `fleet logs -f` and `remote logs -f`
already use: it hands the loop a `context.Context` an interrupt cancels,
and treats that cancellation as a clean end, not a failure.

## Goals / Non-Goals

**Goals:**
- `work logs <id>` prints an item's kept output once, empty where there is
  none yet, refusing an id the API does not carry the way `abort`/`remove`
  already do
- `work logs -f <id>` polls for new output and prints it as it arrives,
  stopping once the item ends (`done`/`failed`) or the operator interrupts
- Reuse `workTarget`/`workRequest`/`followUntilInterrupted` as they stand;
  no change to the API, no new endpoint

**Non-Goals:**
- An offset/range parameter on the log endpoint (`fleet logs -f`'s
  `NextOffset` pattern) — the log endpoint already answers the whole
  content each call, small enough (an item's own kept output) that
  re-fetching it whole each poll is not worth a protocol change for
- A `--limit` flag trimming to the last N lines (`fleet logs`'s own) — an
  item's kept output is one agent's run, not a fleet-wide tail; the
  problem `--limit` solves does not apply
- Interleaving several items' output — `work logs` takes exactly one id,
  like `abort`/`remove` already do

## Decisions

### The follow loop diffs by string prefix, not by offset

Each poll re-fetches the whole log via the same `GET
/v1/items/{id}/log` call a plain `work logs` makes. The loop keeps the
length of what it has already printed; where the new answer's content
still starts with what was printed last time, only the suffix is new and
gets printed. Content that does not start that way (the kept output
disappeared, or changed underneath — neither expected in the item's own
lifetime, but not a case to crash on) is printed in full, as if nothing
had been printed yet: simpler than reconciling a diff against something
the API gives no way to address a byte range of.

### Following waits through the backlog, stops at an ended state

`-f` polls `GET /v1/items` alongside the log, on the same interval, to
read the one item's own `state` (`orchestrator.ItemView`, decoded the way
`work list` already does; the item missing from the list is read the same
as the log call's own "id not carried" refusal). The loop treats
`backlog` and `running` alike — keep polling — and stops once the state
is `done` or `failed`, after one final log poll so nothing the item wrote
right at the end is missed. This is what makes `work logs -f` on an item
that has not started yet a plain "wait for it, then follow" instead of an
immediate end: the proposal's own "the way `tail -f` does" reads more
naturally as "watch this item's output happen" than "only ever look if it
is already running".

### The poll interval is a package var, like `fleetLogsInterval`

`workLogsInterval = 1 * time.Second` — shorter than `fleetLogsInterval`'s
3s, since an item's own agent output is usually what the operator is
actively watching right after starting or aborting it, not a fleet-wide
tail checked in passing. A package var, not a flag, matching
`fleetLogsInterval`'s own precedent: a test overrides it directly rather
than the command needing a hidden flag for it.

### Two API calls per poll, not one

The log endpoint carries no state, and the list endpoint carries no
single item's full log efficiently addressed — combining them into one
new endpoint would be a real API change for a command that otherwise
needs none. Two small calls a second is well inside what a local
orchestrator's work list API is built for (`work list` alone already
polls the same list endpoint from an operator's shell without concern).

## Risks / Trade-offs

- **A poll can catch an item between "done" and its log's last write
  settling.** Not possible here: the orchestrator writes the item's kept
  output before it records the state that ends it (the log is captured by
  the agent's own process exit, the state written after `Wait` returns),
  so by the time a poll sees `done`/`failed` the log call already carries
  everything. The design's own "one final log poll" after the state ends
  is what makes this true from the client's side too, not just the
  server's.
- **Two calls a second, for as long as a follow runs.** Bounded by the
  operator's own patience — a follow that outlives the item ends itself;
  one left running against a long backlog item is no worse than `work
  list` run in a manual polling loop already is.
