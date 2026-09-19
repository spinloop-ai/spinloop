# spinloop logs

What every engine you run has said — through whatever the node answers with, a
daemon's log file or a cloud environment's log store.

```sh
spinloop logs                         # every node in this directory's fleet.yaml
spinloop logs studio                  # just that node
spinloop logs --env prod              # one registered environment, no file needed
spinloop logs -f                      # follow
spinloop logs --env prod --source boot --since 30m
```

With no node named it reads every node in the target; naming one reads only
that one. Nodes are read concurrently, so the command takes as long as the
slowest rather than all of them added up. Lines are prefixed with their node's
name only when more than one node produced output — reading one node reads like
that node's own log.

## Which target

| | target |
| --- | --- |
| `--env <name>` | one registered environment |
| `--fleet <path>` | that fleet file |
| neither | the `fleet.yaml` in the working directory |

**`--fleet` has no `-f` here.** `-f` is `--follow`, as it is on every other
tool that follows something, and a flag cannot carry two meanings on one
command line.

## Which flags apply to which nodes

A daemon's log is one file on one machine, read from a byte offset. A cloud
environment's is a store that can be queried. The three query flags apply to
the nodes that have one and leave the rest read as they would be without them —
so `--source boot` against a fleet of daemons returns their output rather than
nothing.

| flag | cloud environment | daemon node |
| --- | --- | --- |
| `--source engine\|boot\|all` | selects which log | — |
| `--since <duration>` | bounds how far back (default 1h) | — |
| `--instance <id>` | restricts to one instance | — |
| `--follow`, `--limit`, `--format` | applies | applies |

## Following

`-f`/`--follow` keeps printing as output arrives, across every node in the
target at once. A node that goes unreachable mid-follow is reported rather
than dropped silently.

## See also

- [`spinloop status`](status.md) — what each engine is
- [`spinloop metrics`](metrics.md) — what they are doing with the hardware
- [`spinloop serve`](serve.md) — the log of an engine you are running in the foreground
