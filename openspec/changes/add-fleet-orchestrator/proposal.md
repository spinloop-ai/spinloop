## Why

The fleet routes inference, but nothing turns a backlog of work into agents running
against that fleet at a pace the fleet can absorb. The gateway (added 2026-09-12)
removed the last practical obstacle — an agent anywhere now needs only a URL and one
token — so the missing piece is a dispatcher: a process that holds the work items,
decides how many may run at once, and launches the agents. This is issue #188
("feed agents a backlog of work, limiting concurrency to the fleet capacity"),
built on the tag (#190) and concurrency-rule (#189) primitives it references.

## What Changes

- New `spinloop orchestrator` command: a long-running foreground process, beside
  `spinloop gateway`. It holds a backlog of work items, reads the fleet's
  topology from the gateway, admits items while the fleet's concurrency rules
  allow, runs each admitted item as a one-shot agent whose inference is pointed
  at the gateway, and tracks every item to a finished or failed end. It holds
   no `fleet.yaml` and no node tokens: the gateway is its only view of the fleet.
   A `--create-item-dirs` flag (default false) makes a missing item directory
   get created instead of failing the item.
- Work items in v1 come from a file: a list of items, each naming the tags of
  nodes it can run on, the checkout it works in, and the instructions the agent
  is given. A GitHub/Slack item source (issue #152) is a later producer behind
  the same item shape.
- Fleet file: nodes MAY carry tags — key/value string pairs describing what a
  node can take on. Tags are claims the operator makes; they are not read by any
  node and never change what an engine runs.
- Fleet file: a top-level concurrency section declaring how much work the fleet
  may take — a limit per tag (items carrying that tag, fleet-wide) and a limit
  across all active nodes. The orchestrator admits an item only while every
  limit the item counts against has room; the limits are the fleet's declared
  capacity, not a measurement of load.
- The gateway serves the fleet's topology over a new read endpoint, authenticated
  like everything else it serves: each node's name, kind, tags, state, served
  model (or, when stopped, the model its own source would start), readiness, and
  last-active, plus the file's wake policy, preference, and concurrency rules.
  The gateway's fleet file stays the single source of truth for topology.
- Nodes stay what they are: inference capacity. No daemon endpoint, no remote
  control-plane change, no work-item state on any node. The gateway's existing
  per-request wake is the only thing that ever starts an engine on a node,
  including for an agent the orchestrator launched.

## Capabilities

### New Capabilities

- `fleet-orchestrator`: the `spinloop orchestrator` command — the work item
  shape and its file source, admission against the concurrency rules, tag
  matching and node selection, one-shot agent dispatch through the gateway,
  item lifecycle and state, and what it deliberately does not do.

### Modified Capabilities

- `fleet-config`: a node entry MAY name tags (key/value string pairs); the file
  MAY declare a concurrency section (per-tag limits and a fleet-wide limit)
  with validation.
- `fleet-gateway`: the gateway serves the fleet's topology over a read endpoint
  behind its caller authentication — the nodes with their tags and serving
  facts, and the file's fleet-level settings.

## Impact

- `internal/fleet`: the fleet file parses node tags and the concurrency section;
  both are plain data the gateway reports and the orchestrator consumes.
- `internal/gateway`: a new topology endpoint over the cached status fan-out it
  already runs, joined with the fleet file's tags and settings; `docs/openapi.yaml`
  unchanged (the endpoint is the gateway's own surface, not the daemon's).
- `internal/orchestrator` (new): the work item and its file source, the pure
  admission and selection logic, the state the process keeps, and the dispatch
  loop that launches one-shot agents.
- `cmd/spinloop`: the new command, its flags, and its place in help and
  completion; dispatch reuses the launch path's apply-and-exec rather than
  reimplementing it.
- `docs/`: the orchestrator command reference and the fleet file's two new
  sections; `examples/` gains an orchestrator scenario beside the gateway one.
- No daemon endpoint changes, no remote control-plane changes, no new
  dependencies. A fleet file with no tags and no concurrency section, and a
  gateway serving no orchestrator, behave exactly as they do now.
