# Design: the orchestrator's work list API

## Context

The run loop today opens a concrete `Store` (`internal/orchestrator/state.go`)
— the state file's path, the logs directory, the lock file, and the pid —
holds the state map and the in-flight set as locals in `Run`, and saves after
every pass. The lock is a pid file: a second process for the same items file
is refused, so nothing outside the run can share the store. The items file
itself is re-read from disk when its mtime or size changes.

The server shape to copy is `spinloop gateway`: `internal/gateway` holds the
`Handler`, `New`, and a `Listen(addr, token)` that refuses a non-loopback
bind with no token, beside `DefaultListen` / `LoopbackListen`; the command
adds `--listen`, `-l/--loopback`, `--api-token`, `--api-token-file` (the
token resolved the daemon's way), puts every path — `/health` included —
behind a constant-time bearer check, and answers an unknown method or path
`404` naming the paths it serves.

See proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**

- A `Store` interface in `internal/orchestrator` with the file-backed store
  as the default implementation, so the run and the API both work through
  the same seam and tests need not put state on disk.
- A work list API served by the orchestrator process, gateway-shaped:
  address and token flags, the loopback affordance, bearer auth on every
  path, and the same exposure rule — query and mutate the work list from
  another client.
- One owner of the mutable work list — records, in-flight, and the items
  view — so the loop and the API cannot race each other over the files.

**Non-Goals:**

- The `work` commands (#206-#209). They are this API's first clients and
  build on it; none of them exists yet.
- A second store implementation. The interface is the plug; the file store
  stays the only socket, and nothing user-facing selects a store.
- Multi-process sharing of one work list. The lock keeps refusing a second
  orchestrator; a shared backend is what the interface would allow later,
  not what this change builds.
- Any change to the fleet gateway's surface or to the items file's format.

## Decisions

### The API lives in the orchestrator process

The mutations the API takes need what only the run's process holds: the
in-flight children (an abort stops one), the lock (a second opener is
refused by design), and the run's live view of the items file. A separate
server process could not open the store the file lock protects, could not
stop a child it did not launch, and would have to invent a control channel
to the run for what the in-process API does by calling a function.

The precedent is the family's own: `spinloop gateway` and
`spinloop serve --daemon` are long-running foreground commands that serve
their own APIs. `spinloop orchestrator` becomes the third.

Alternatives considered: a separate `work serve`-style command (blocked by
the lock and the children, as above); a store backend that processes share
over a socket (builds a coordination system for a requirement that does not
exist yet).

### The Store is the seam; the shared state is a core, not the store

`Store` names what the file store does today: the per-item records
(load/save), each item's log path, the lock's lifecycle (open, refuse a
live holder, take over a stale one, release), and the recovery a restart
owes — an item left running is recorded failed, naming the interruption.
The file implementation is today's `state.go` behind that interface, its
paths and shapes unchanged.

What the loop and the handlers must share is more than persistence: the
in-flight set (which item has which child) and the current items view. That
sharing is a small core the run builds — the state map, the in-flight set,
and the items view under one mutex — with the loop's pass and each API
operation as functions on it. The API's add, remove, and abort and the
loop's admit and reap take the same lock; an abort stops its child through
the core, not through a file. The items view refreshes when the loop
re-reads a changed file and when the API itself writes one, so a read
straight after an add shows the add.

The interface rather than a shared struct, for two reasons: the struct
today is a file layout, and the store is the last piece of the run a test
must put on disk. And "pluggable" is the stated shape of the seam — a
second backend slots in behind it without touching the run or the API.

### The items file stays the source of truth for items

The operator edits the items file by hand today and the loop re-reads it on
change, so the API's add and remove write back to that file — parse,
change, validate, write — rather than holding a private copy of the items.
A restart then sees exactly what the API did, and the state file keeps
owning only the records, the logs directory only the output, and the file
store's paths change for nothing.

Alternatives: the core owns the items in memory and the file is a cache
(two sources that can disagree about what the file says); or the API
reports items straight from the file and leaves mutations to the client
(editing the file by hand) — the status quo this change removes.

### The API's surface mirrors the work family's issues

- `GET /v1/items` — the work list: every item with its record, the file's
  order.
- `GET /v1/items/{id}/log` — the output the item's agent kept; none
  answered as none.
- `POST /v1/items` — add an item, the file's validation applied, including
  the refusal of an id the state already records as done or failed.
- `DELETE /v1/items/{id}` — remove the item, its record, and its output; a
  running item is refused, naming it and saying to abort it first.
- `POST /v1/items/{id}/abort` — stop the item's agent the way a clean
  interrupt stops it and return the item to the backlog.
- `/health`, and a `404` naming the served paths for the rest.

`work list` (#207) reads `GET /v1/items`; the other commands each map to
one path. They are clients of the API, not co-writers of the files, so the
refusals the issues specify live in one place.

### The server mechanics are the gateway's, port for port

`DefaultListen` is `:4010` and `LoopbackListen` is `127.0.0.1:4010` — the
gateway is `:4000` (the docker e2e's cold gateway takes `:4001`), the daemon
`:4242`, and the orchestrator takes its own address in the family. The command takes `--listen`, `-l/--loopback`,
`--api-token`, and `--api-token-file`; the token is resolved the daemon's
way (a literal, a file, or `SPINLOOP_API_TOKEN`). `Listen` refuses a
non-loopback bind with no token; an explicit address and `--loopback`
together fail, naming both; every path, `/health` included, sits behind the
bearer; an unknown method or path is `404` naming the surface.

The server is up before the run's first pass, the gateway's order — the
handler in before a signal can arrive — and it shuts down with the run's
clean interrupt, after the agents are stopped and their items back in the
backlog, so an in-flight request cannot outlive the state it touched.

The new token is the API's own — the token callers present to the
orchestrator. It is distinct from `--token-env`, the variable holding the
token the orchestrator presents to the fleet's gateway: the same shape, two
ends of the run.

## Risks / Trade-offs

- [Two orchestrators for different items files on one machine both bind
  `:4010`] → the second bind fails with the listener's error naming the
  address, the way a second gateway does; the operator passes `--listen` or
  `--loopback` as they do today for the gateway.
- [An operator saving the items file in the same instant the API writes it]
  → both are plain writes to the file the run already re-reads on change;
  the same race as two editors today, no new lock buys anything.
- [The API puts an agent's output on the network] → the exposure rule is
  the daemon's and the gateway's: a non-loopback bind without a token is
  refused, and loopback needs none.
- [An abort lands on a child the loop is about to reap] → one core lock:
  the abort removes the in-flight entry and stops the child under it; the
  reap finds no entry and skips, the way it already skips an id it has
  reaped.

## Migration Plan

No data moves: the state file, the logs, and the lock keep their paths and
shapes, and a state an older orchestrator left is what a new one opens.
Rollback is reverting the change; nothing it writes is read by anything
older.

## Open Questions

None that block implementation. The `work` commands' output formatting is
#207's to decide; the abort's stop grace reuses the run's existing one.
