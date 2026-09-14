## Context

See proposal.md for the motivation. What matters for the approach is what
already exists:

- The gateway is the fleet's front door: it holds the fleet file, every node's
  token and engine key, a short cache over the status fan-out, and the
  "what a request could start" computation over each node's own source
  (the `/v1/models` listing). Its design deliberately excluded queueing,
  budgets, and anything stateful: "a binary to run, not a service to operate".
- The launch path (`applyBeforeLaunch`/`launchAgent` in `cmd/spinloop`) is the
  one existing place that turns "a model, a base URL, and a key variable" into
  "an agent that runs": it applies the selection into the harness's config and
  execs the harness with the environment filled. A gateway-routed launch
  already produces exactly the shape an orchestrator dispatch needs — base URL
  the gateway, key the gateway's token.
- `internal/fleet`'s selection, waking, and readiness machinery is written
  once and consumed by the launch path and the gateway. It routes inference;
  nothing in it runs an agent, and nothing changes in it for this.
- The decisions this design takes as given (made with the owner): the
  orchestrator is a separate process from the gateway and may run on a
  different machine; it learns the fleet's topology from the gateway rather
  than holding a second fleet file; a work item names the tags of nodes it
  can run on, not a model; and the orchestrator holds the items in flight —
  a node is inference capacity and carries no work-item state.

## Goals / Non-Goals

**Goals:**

- A `spinloop orchestrator` that works a backlog at the pace the fleet's file
  declares, using only the gateway as its fleet view.
- The admission and matching logic as pure, testable functions: given the
  topology, the limits, and the in-flight set, what may be admitted and to
  which node — the same shape as `internal/fleet`'s pure `choose`/`rank`.
- Dispatch built on the launch path's apply-and-exec, not a second mechanism
  for pointing an agent at a model.

**Non-Goals:**

- Anything on a node: no daemon endpoint, no remote control-plane change, no
  work-item state anywhere but the orchestrator's own process and files.
- Result publishing (a pull request, a comment on the source issue or
  thread) and the GitHub/Slack item source itself: issue #152's scope, a later
  producer behind the same item shape.
- Multiple orchestrators working one backlog, and auto-retry of failed items.
- Guaranteeing which node the gateway serves each request to: the limits bound
  how many items are admitted, and the gateway keeps its own per-request
  routing.

## Decisions

### A top-level `orchestrator` command beside `gateway`, a new `internal/orchestrator` package

`spinloop orchestrator --gateway <url> [--items ./work.yaml] [--token-env VAR]`
runs in the foreground the way `gateway` does: its lifecycle is the
process's. Top-level rather than `fleet orchestrator`, for the same reason the
gateway is: it does not observe the fleet the way `fleet status` does; it
works a backlog through a gateway, which is a different job, and it holds no
fleet file at all.

Configuration is flags plus the items file, in keeping with the gateway
decision that "fleet.yaml plus flags is the configuration" — here, "a gateway
address plus an items file is the configuration". The token resolves under
`OPENAI_API_KEY` by default, the fleet file's gateway-section convention.

The item model, its file source, the pure admission/matching logic, the state
file, and the dispatch loop live in `internal/orchestrator` as an
`http`-free core that talks to the gateway through a small interface, so the
loop is unit-testable against a fake topology the same way the fleet client
is tested against a fake node.

### The gateway serves the topology at a read endpoint; the run holds no fleet file

`GET /v1/fleet` on the gateway, behind its caller authentication, answers
with the nodes (name, kind, tags, state, served model and name, readiness,
last-active, and — for a stopped node whose source describes a model and the
fleet wakes — the model a request would start it with) plus the file's wake
policy, preference, and concurrency limits, each absent where the file
declares it not.

This is mostly assembly of what the gateway already holds: the cached status
fan-out (the same two-second cache `/v1/models` reads), the per-node source
resolution the model listing performs, and the fleet file's new fields. The
orchestrator polls it on its tick; the gateway's cache keeps the poll cheap
and the orchestrator's view consistent with the one the gateway routes on.

The command reads the fleet file only where the operator names no gateway
flag: for the section's address, the section's token variable where the
token flag was not given, and the token's value through the file's own
chain — the environment first, then the `.env` beside the file, the way
every other secret this file references resolves. Nothing past that crosses
into the run — the file is a pointer to the front door, not a copy of what
is behind it.

Alternatives, both rejected: the orchestrator holding its own fleet file and
driving `internal/fleet` directly (a second copy of the topology, and the
drift the owner ruled out), and the orchestrator fanning out to the daemons
itself (it would need every node's token, undoing the gateway's point that
secrets stop at one process).

### Tags and limits are fleet-file data the gateway reports, not daemon facts

Node `tags` (a key/value map, duplicate keys refused) and the `concurrency`
section (`total`, plus per-tag limits keyed by the `key=value` form) parse in
`internal/fleet` beside `wake` and `prefer`, with the same validation
posture: a new optional section, a file without one unchanged, bad values
refused at parse time naming the entry. No node reads either: a tag is the
operator's claim about what work a node takes, and a limit is the operator's
declaration of how much the fleet absorbs. The daemon's reported facts —
state, served model, readiness, last-active — stay facts, and the topology
joins the two, so a tag that has gone stale next to a changed served model is
visible in the reply rather than hidden.

### The orchestrator's loop: tick, admit, launch, reap

One process, one loop:

```
every tick (a few seconds, internally fixed for v1):
  topology ← gateway /v1/fleet          (fails → stop the command, per spec)
  re-read the items file if it changed  (new items enter the backlog)
  for each item, highest priority first:
     node   ← match(item.tags, topology)        pure
     room   ← limits admit? (per tag carried + total)  pure
     if node and room: launch(item, node) → in-flight
  reap finished children → done/failed, free their slots
```

Matching, ranking (running-and-answered before a node to be started, then the
file's preference), and the admission arithmetic are pure functions over
(item, topology, limits, in-flight), unit-tested directly. Waiting is not
failing: an item no node matches, or no limit has room for, simply is not
admitted this pass.

### Dispatch reuses the launch path's apply-and-exec

An admitted item runs as the active harness in its non-interactive
single-task form (opencode's `run`, given the model and the item's
instructions; the form is a per-harness table in the dispatch layer, and a
harness without one fails the launch naming the harness). The agent works in
the item's directory with its output kept per item beside the items file.

The directory check at the top of the launch creates a missing directory
(through `MkdirAll`) only where the command set its `--create-item-dirs`
flag — the flag rides into the dispatcher at construction, and a directory it
cannot create fails the item naming it and the cause.

Pointing the agent at the model follows the launch path's own shape: the
selection is synthesized from the gateway address and the chosen node's model
name (served name where the node reports one, else the model id — the same
name the gateway matches on), and the provider is applied into the harness's
config the way `spinloop harness` applies one, under a lock so concurrent
dispatches merge serially. The provider entry the orchestrator adds names the
gateway's OpenAI-compatible address — the gateway's address with the `/v1`
prefix added where it names no path of its own, `fleet.EndpointBaseURL`, the
same conversion a gateway-routed launch applies — and the token variable,
never a value; it is added, not removed — removal stays the operator's
`unapply`, as the launch path's is. The child
is run with its working directory the item's and its output captured, rather
than stdio forwarded, since no person is at the keyboard.

### State lives beside the items file; a restart never re-runs an item

The orchestrator keeps `<items-file>.state.json` (each item's state:
backlog, running, done, failed, and why) and one log per item under
`<items-file>.logs/`. On start, an item the state left running is recorded
failed, naming the interruption, and is not re-run: the agent it launched is
gone, and re-running work whose effect is unknown is worse than a recorded
failure. On a clean interrupt the orchestrator stops its children and puts
their items back in the backlog instead, so a deliberate stop loses nothing.
Failed items stay failed: no auto-retry in v1, the operator acts.

The items file remains the backlog's source of truth for what exists; state
is the orchestrator's record of what happened to each. An item dropped from
the file keeps its recorded end; an item added to it enters the backlog on
the next pass.

## Risks / Trade-offs

- **The orchestrator's host does every agent's work** — checkouts, builds,
  tests all run on the machine the orchestrator runs on, and that machine is
  the throughput ceiling. → Accepted: it is the price of nodes staying pure
  inference capacity, and the issue's own framing caps the fleet, not a
  compute pool. Separate worker hosts are the later extension.
- **Per-tag limits count items, not tokens** — two nodes can serve one model,
  and the gateway routes each request by the model's name, so a limit on
  `model=x` bounds how many items are admitted, not which nodes serve them.
  → Accepted as the honest v1: the limits are the declared capacity the
  operator set, and an operator who wants strict per-node isolation gives
  nodes distinct models or tags. A node hint in the gateway is the later
  escape hatch.
- **An item matched to a stopped node sits on its first token** — the
  gateway holds the agent's first request while the engine loads, and a cold
  wake is minutes. → The item counts in-flight while it waits, so the limits
  throttle cold starts for free; a wake that times out fails the request the
  way it does for any caller, and the item's end follows the agent's.
- **The harness config is shared with whatever else runs on the machine** —
  the orchestrator merges provider entries into the same config a person's
  own launches use. → The merges are the idempotent read-modify-write the
  repo already relies on, serialized inside the process; the orchestrator is
  aimed at a machine (or a user) set aside for it, and the docs say so.
- **One orchestrator per items file** — state is local, and a second process
  on the same file would double-run items. → Refused at start: a lock file
  beside the state refuses a second orchestrator for the same items file,
  naming the first.
- **The gateway is a single point of view** — it goes down and the
  orchestrator stops working the backlog, per the command's own contract. →
  Items are safe in the file and the state; the orchestrator restarts when
  the gateway returns. Nothing is lost, and no item runs twice.

## Migration Plan

Additive throughout. A fleet file with no `tags` and no `concurrency` parses
and behaves exactly as it does now; a gateway serving no orchestrator gains
one read endpoint and nothing else; the launch path, the daemon, and the
remote control plane are untouched. Rollback is removing the orchestrator
process and, if wanted, the file's new sections; nothing else changes
behaviour unless something asks it to.

## Open Questions

- The exact single-task form of each harness beyond opencode (its `run`):
  confirmed per harness as the dispatch table is written; a harness without
  one is a named launch failure, not a blocker for opencode-first use.
- Whether the per-item log keeps the agent's full output or a bounded tail:
  the spec requires the output be kept; the bound is an implementation
  detail.
