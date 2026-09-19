# spinloop metrics

What every engine you run is doing with its hardware — resource use, token and
request counters, and what the node it runs on is.

```sh
spinloop metrics                      # the fleet.yaml in this directory
spinloop metrics --fleet ./cluster.yaml
spinloop metrics --env prod           # one registered environment, no file needed
spinloop metrics -w                   # redraw in place every 60 seconds
spinloop metrics --cost               # add what each priceable node has spent
```

## Which target

The three ways every command that reads engines takes one:

| | target |
| --- | --- |
| `--env <name>` | one registered environment |
| `--fleet <path>` (`-f`) | that fleet file |
| neither | the `fleet.yaml` in the working directory |

`--env` and `--fleet` name two different things, so passing both fails saying
so. With none of the three resolvable the command fails naming all of them.

## Formats

`--format` selects one of four. All four report the same facts; they differ in
how much of the screen they spend.

| format | what it draws |
| --- | --- |
| `gauge` (default) | a header line per node, then the current reading as gauges |
| `bar` | the same, with each series as a sparkline of the retained history |
| `table` | every fact on its own line, then the counters and figures |
| `json` | one object per node, for a program to read |

`table` is the one to use when you want everything a node can say:

```
node:         prod
state:        running
instance:     i-0abc123
instanceType: g6e.xlarge
runner:       llamacpp
model:        org/qwen3-27b
version:      1.41.0
endpoint:     http://198.51.100.1:8000/v1
uptime:       2h 15m 0s
active:       2m 5s ago

  running:          1
  prompt tokens:    12000
  …
```

## Which flags apply to which nodes

Some facts only one kind of node has. A flag that asks for one applies to the
nodes that can answer it and leaves the rest exactly as they read without it —
a fleet of daemons asked for a cost shows no cost and still succeeds.

| flag / field | cloud environment | daemon node |
| --- | --- | --- |
| `--cost` | priced from its instance type and region | — |
| `instance`, `instanceType` | reported | — |
| `version` | reported | — |
| `endpoint` | reported | — |
| everything else | reported | reported |

A daemon node runs on a machine you already own: it has no hourly rate, no
instance id, and no address the control plane publishes. Its release is on its
[status](status.md) row instead.

## Watching

`-w`/`--watch` redraws every 60 seconds, clearing the screen each time rather
than accumulating scrollback, and exits cleanly on interrupt. For an
interactive view that also drives the nodes, use
[`spinloop dashboard`](dashboard.md).

## See also

- [`spinloop status`](status.md) — what each engine is, one row each
- [`spinloop logs`](logs.md) — what they have said
- [`spinloop dashboard`](dashboard.md) — these readings, live and interactive
