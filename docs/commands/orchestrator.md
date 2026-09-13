# spinloop orchestrator

Work a backlog of items against a [fleet](fleet.md) at a pace the fleet can
absorb. The orchestrator holds the backlog, reads the fleet's topology from
the fleet's [gateway](gateway.md), admits an item while the fleet's declared
[concurrency](fleet.md#concurrency) limits allow, and runs each admitted item
as a one-shot agent of the [active harness](harness.md) in the item's own
directory — the agent's inference going through that same gateway.

```sh
spinloop orchestrator --gateway http://gateway.internal:4000
spinloop orchestrator --gateway http://gateway.internal:4000 --items ./work.yaml
spinloop orchestrator --gateway http://gateway.internal:4000 -H pi
```

It runs in the foreground, the way [`spinloop gateway`](gateway.md) does: the
signal is the only exit, and a clean interrupt stops its agents and puts their
items back in the backlog — none lost, none run twice. An item that has ended
is not run again on a restart.

It takes no fleet file: the gateway is its only view of the fleet, and it holds
no node token and no engine key. What it holds is one credential — the
gateway's bearer token, from the environment variable `--token-env` names
(`OPENAI_API_KEY` by default) — and that token reaches each agent as its key.

## The items file

A list of items, each with an id, the instructions its agent is given, and the
directory the agent works in:

```yaml
- id: fix-the-parser
  instructions: fix the failing tests
  dir: ./parser

- id: bump-the-deps
  instructions: run the dependency upgrade and resolve the breakages
  dir: ./
  priority: 10          # optional; higher first
  tags:                 # optional; every one must be a tag of the node it runs on
    - gpu=a100
```

- **`id`** — unique in the file; the state and the per-item log are keyed by it.
- **`instructions`** — the task the agent is given, whole.
- **`dir`** — the working directory the agent runs in. A missing directory
  fails that item and only that item; the rest of the backlog goes on.
- **`priority`** — an integer, higher first; items of one rank go in file
  order.
- **`tags`** — `key=value` pairs naming the [tags](fleet.md#tags) of the nodes
  the item may run on. Every pair must be a tag the node carries; an item with
  no tags may run anywhere. An item nothing matches waits — the fleet may
  change — rather than failing.

The file is re-read while the orchestrator runs: a new item enters the
backlog, and an item dropped from the file keeps its recorded end.

## What it keeps

Beside the items file, and only beside it:

```
work.yaml            the items
work.yaml.state.json the record: one entry per item that is running or has ended
work.yaml.lock       what keeps a second orchestrator off the file
work.yaml.logs/      one log per item, its agent's output
```

The state holds each item's `running`/`done`/`failed` record — the node a
running one is on, the reason a failed one failed — and an item with no entry
is in the backlog. A run that ends in a crash leaves its in-flight items
recorded `running`; the next start for the same file records them `failed`,
naming the interruption, and does not run them again. A clean interrupt does
the re-queueing itself: its items simply lose their records.

One orchestrator per items file: a second one for the same file is refused,
naming the holder.

## How an item runs

An admitted item is launched as the harness's non-interactive single-task
form — `opencode run` or `pi --print` — in the item's directory. The model it
runs against is the gateway and the chosen node: the node's model, under a
provider the orchestrator writes into the harness config for the run, with the
gateway's address as the base URL and the gateway's token as the key. A
harness with no single-task form — lucinate — is refused at startup, naming it.

The node an item takes is the fleet's own routing, read through the gateway:
an item matches a node only where every tag it names is one the node carries;
a node already running and answering is offered before a node the run would
have to start, and among a tier the fleet file's `prefer` ranks them. A
stopped node is an option only where the fleet file
[wakes](fleet.md#waking).

## What it does not do

- It does not start or stop anything on a node. A stopped node appears in the
  topology with the model the fleet would start it with, and an item may be
  admitted against it — the engine is started by the fleet's own machinery,
  the same as a gateway request would.
- It holds no fleet file and no node credential. The topology is its whole
  view, and a gateway that stops answering ends the run, naming the gateway —
  the items are safe in the file and the state, and a restarted run picks
  them up.
- It never re-runs an ended item. `done` stands, and a `failed` one —
  including the ones a crash left in flight — stands.
- It is not the fleet's scheduler. The limits it works to are the fleet
  file's declared [concurrency](fleet.md#concurrency); it measures nothing and
  estimates no load.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `--gateway <address>` | The fleet's gateway — required. The topology, and the agents' inference, both go through it |
| `--items <path>` | The work items file (default `./work.yaml`) |
| `--token-env <variable>` | The environment variable holding the gateway's bearer token (default `OPENAI_API_KEY`) |
| `-H`, `--harness <name>` | Which harness to run the agents with (default the resolved one) |
| `--log-level` | `debug`, `info`, `warn`, or `error` — overrides `SPINLOOP_LOG_LEVEL` (default `info`) |

## See also

- [Working a backlog against the fleet](../work-items.md) — the feature this command drives, and how to work the items file
- [`spinloop gateway`](gateway.md) — the endpoint the orchestrator reads, and the one its agents call
- [`spinloop fleet`](fleet.md) — the file the fleet's tags and concurrency limits live in
- [`spinloop harness`](harness.md) — the single-task form each item runs as
