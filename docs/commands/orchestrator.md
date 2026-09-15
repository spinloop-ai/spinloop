# spinloop orchestrator

Work a backlog of items against a [fleet](fleet.md) at a pace the fleet can
absorb. The orchestrator holds the backlog, reads the fleet's topology from
the fleet's [gateway](gateway.md), admits an item while the fleet's declared
[concurrency](fleet.md#concurrency) limits allow, and runs each admitted item
as a one-shot agent of the [active harness](harness.md) in the item's own
directory — the agent's inference going through that same gateway.

```sh
spinloop orchestrator
spinloop orchestrator --gateway http://gateway.internal:4000
spinloop orchestrator --gateway http://gateway.internal:4000 --items ./work.yaml
spinloop orchestrator --fleet ./fleet.yaml -H pi
```

It runs in the foreground, the way [`spinloop gateway`](gateway.md) does: the
signal is the only exit, and a clean interrupt stops its agents and puts their
items back in the backlog — none lost, none run twice. An item that has ended
is not run again on a restart.

While it works, it serves the [work list API](#the-work-list-api) on its own
listener, the gateway's server pattern: an address and a token, and loopback
the bind that needs neither beyond the machine.

Where no `--gateway` is given, it reads the [fleet file](fleet.md) — the one
`--fleet` names, or `./fleet.yaml` — for the gateway's address and the
section's token variable, and nothing else: the gateway is the run's only view
of the fleet, and it holds no node token and no engine key. What it holds is
one credential — the gateway's bearer token, from the environment variable
`--token-env` names, or the section's `tokenEnv` where the gateway comes from
the file and no flag is given (`OPENAI_API_KEY` by default); where the
gateway comes from the file, the value is read the way the file reads its
secrets — the environment first, then the `.env` beside it — and that token
reaches each agent as its key.

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
  fails that item and only that item; the rest of the backlog goes on. With
  `--create-item-dirs`, the orchestrator creates a missing directory instead.
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
work.yaml.aborts/    the abort markers left beside the file, while they stand
```

The state holds each item's `running`/`done`/`failed` record — the node a
running one is on, the reason a failed one failed — and an item with no entry
is in the backlog. A run that ends in a crash leaves its in-flight items
recorded `running`; the next start for the same file records them `failed`,
naming the interruption, and does not run them again. A clean interrupt does
the re-queueing itself: its items simply lose their records.

One orchestrator per items file: a second one for the same file is refused,
naming the holder.

## The work list API

The work list — every item the items file carries, joined with the run's
record for it — is queryable and mutable over HTTP while the run works, so a
client can watch the backlog move and act on it without touching the file. The
run and the API are one pair of hands on the same state: a change the API
accepts reaches the run on its next pass, and a pass the run takes shows in the
list before it returns.

```
Working work.yaml against http://gateway.internal:4000: 3 items in the backlog
Work list on 127.0.0.1:4010
```

| Path | Meaning |
| ---- | ------- |
| `GET /health` | That the orchestrator is up. It touches no file and no work on purpose — it is how you tell the orchestrator down from the fleet down. |
| `GET /v1/items` | The work list: every item the file carries, in the file's order, each with its record — the item's fields, its `backlog`/`running`/`done`/`failed` state, the node a running one is on, when it started and ended, and the reason a failed one failed. An item with no record is `backlog`. |
| `GET /v1/items/{id}/log` | The item's kept agent output, as it was kept. An item that wrote none is answered as having none (`"log": null`), not as a fault. |
| `POST /v1/items` | Add an item to the file and the backlog: the file's validation on its fields, and the file stays a valid items file after the write. |
| `DELETE /v1/items/{id}` | Take an item out of the work list: the items file, its record, and its kept output, all of it. |
| `POST /v1/items/{id}/abort` | Stop a running item's agent the way a clean interrupt stops it — the polite signal, the grace, then the hard end — and put the item back in the backlog, where the run admits it again on a later pass. |
| Any other path or method | A `404` naming the paths the API serves. |

The mutations refuse rather than force:

- An **add** is refused a `400` where the item's fields fail the file's
  validation; a `409` where the file already carries the id — naming it — or
  the state records it ended, `done` or `failed` — naming the record.
- A **remove** is refused a `409` where the item is running — naming it and
  the abort that goes first — and a `404` where the file does not carry the id,
  naming it.
- An **abort** is refused a `409` where the item is not running — naming the
  item and its state — and a `404` where the file does not carry the id,
  naming it.

### The API's token

Callers present the API's token as a bearer token on every request,
`/health` included — a wrong or missing one is a `401`. The token comes from
one of three places, the same rules the
[daemon's](serve.md#the-control-api---api-and-spinloop-daemon) token follows,
and giving two at once is an error rather than a silent precedence:

| Source | Notes |
| ------ | ----- |
| `--api-token-file <path>` | The file's contents, trimmed. |
| `SPINLOOP_API_TOKEN` | The environment. |
| `--api-token <value>` | The token itself — readable by every local user through `ps`, like the daemon's. |

A non-loopback listen with no token refuses to start, naming the address and
the three ways to supply one; a loopback listen (`-l`, or `--listen
127.0.0.1:4010`) needs none. The default, `:4010`, binds every interface.

The API's token is a separate credential from the
[gateway's](gateway.md#the-gateways-token): the gateway's token — the one
`--token-env` names — is what the orchestrator presents *to* the gateway, and
what each agent holds as its key; the API's token is what a caller presents *to
the orchestrator*. One machine can run the two with different values, and the
API's token never reaches a node or an agent.

## How an item runs

An admitted item is launched as the harness's non-interactive single-task
form — `opencode run` or `pi --print` — in the item's directory. The model it
runs against is the gateway and the chosen node: the node's model, under a
provider the orchestrator writes into the harness config for the run, with the
gateway's OpenAI-compatible address as the base URL — its address plus `/v1`
where it names none — and the gateway's token as the key. A
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
- The run holds no fleet file and no node credential: the file, where read,
  gives the gateway's address and nothing else. The topology is its whole
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
| `--gateway <address>` | The fleet's gateway; where not given, the fleet file's gateway section supplies it. The topology, and the agents' inference, both go through it |
| `-f`, `--fleet <path>` | The fleet file to find the gateway in, where `--gateway` is not given (default `./fleet.yaml`) |
| `--items <path>` | The work items file (default `./work.yaml`) |
| `--create-item-dirs` | Create an item's working directory if it does not exist (default off) |
| `--token-env <variable>` | The environment variable holding the gateway's bearer token (default `OPENAI_API_KEY`, or the fleet file's section where the gateway comes from it and no flag is given; the value, on that path, also from the `.env` beside the file) |
| `--listen <address>` | The address to serve the work list API on (default `:4010`) |
| `-l`, `--loopback` | Serve the work list API on loopback on the default port (`127.0.0.1:4010`); needs no token |
| `--api-token-file <path>` | Read the work list API's bearer token from this file |
| `--api-token <value>` | The work list API's bearer token |
| `-H`, `--harness <name>` | Which harness to run the agents with (default the resolved one) |
| `--log-level` | `debug`, `info`, `warn`, or `error` — overrides `SPINLOOP_LOG_LEVEL` (default `info`) |

## See also

- [Working a backlog against the fleet](../work-items.md) — the feature this command drives, and how to work the items file
- [`spinloop gateway`](gateway.md) — the endpoint the orchestrator reads, and the one its agents call
- [`spinloop fleet`](fleet.md) — the file the fleet's tags and concurrency limits live in
- [`spinloop harness`](harness.md) — the single-task form each item runs as
