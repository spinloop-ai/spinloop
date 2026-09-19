# spinloop dashboard

The live, interactive view of what you are running: a tile per node, repainted
on an interval, with the keys to drive the one you have selected.

```sh
spinloop dashboard                       # the fleet.yaml in this directory
spinloop dashboard --fleet ./cluster.yaml
spinloop dashboard --env prod            # one environment, as a board of one
```

Each tile draws what the gauge format of
[`spinloop metrics`](metrics.md) prints for that node — its state,
what it serves, the resource gauges, the token counters — so the board and the
metrics view cannot word the same reading differently.

## Keys

| key | what it does |
| --- | --- |
| `↑ ↓ ← →` | move the selection |
| `enter` | open the selected node's detail view |
| `s` | start the selected node's engine |
| `k` | keep a cloud environment for a duration you type (asks how long, pre-filled `4h`) |
| `a` | abandon a start still in flight — the wait ends, the node is free; a wake the cloud is already carrying goes on |
| `x` | stop the selected node, after a confirmation |
| `r` | force a refresh |
| `q` / `ctrl-c` | leave |

`k` shows only for a node that can be kept — a cloud environment — and a kept
environment carries its deadline beside the last-active line on both the tile
and the detail view, whatever the engine's state.

## Which target

The three ways every command that acts on a fleet takes one — `--env <name>`,
`--fleet <path>`, or the working directory's `fleet.yaml`. A single
environment opens as a board of one panel, which needs no fleet file at all.

A problem with the target itself — a missing or unparseable fleet file, an
environment that is not registered — fails before the view opens. A problem
with any *node* does not: an unreachable node is a tile showing why, and a
node whose token reference cannot be resolved holds its reason for the life of
the view.

The board needs a terminal. To stream the same readings into a pipe, use
`spinloop metrics --watch`.

## See also

- [`spinloop status`](status.md) — the same facts as one table, scriptable
- [`spinloop metrics`](metrics.md) — the same readings as text
- [`spinloop fleet`](fleet.md) — the fleet file, and driving nodes from the CLI
