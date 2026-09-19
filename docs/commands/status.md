# spinloop status

What every engine you run is doing, one row each — whether that is a fleet of
machines, a single cloud environment, or both.

```sh
spinloop status                       # the fleet.yaml in this directory
spinloop status --fleet ./cluster.yaml
spinloop status --env prod            # one registered environment, no file needed
```

```
NODE     STATE         SERVING
studio   running       llamacpp  qwen3-27b  (up 2h 14m)  (active 3m ago)  (1.40.0)
gpu-box  stopped
prod     running       vllm  org/model  (active 12s ago)
dead     unreachable   dial tcp 198.51.100.9:4242: connect: connection refused
```

Nodes are queried concurrently, so the command takes as long as the slowest
one rather than all of them added up. A node that cannot be reached is a row
saying why, not a failure: one unreachable machine never blanks the rest, and
the command still succeeds.

## Which target

The same three ways every command that acts on a fleet takes one:

| | target |
| --- | --- |
| `--env <name>` | one registered environment |
| `--fleet <path>` (`-f`) | that fleet file |
| neither | the `fleet.yaml` in the working directory |

`--env` and `--fleet` name two different things, so passing both fails saying
so. With none of the three resolvable the command fails naming all of them —
it does not go looking for an engine on the machine you are sitting at. To
watch a local engine, [`spinloop serve`](serve.md) shows the one it runs; to
read it from elsewhere, run [`spinloop daemon`](serve.md#the-control-api-api-and-spinloop-daemon)
and name that machine in a `fleet.yaml`.

## What a row says

- **state** — `idle`, `running`, `stopped` or `crashed`, or the reason the
  node did not answer.
- **serving** — the runner and model, then how long it has been up, how long
  since it last did work, and the spinloop version of the daemon running it.
- **not ready** — the engine's process exists but it has not answered its own
  health check, so it is not servable yet however long its uptime says. A
  cloud environment whose endpoint the control plane reports unhealthy reads
  the same way.

Two things are deliberately not columns, because only one kind of node has
them: an environment's endpoint address, which is
[`spinloop remote env`](remote.md), and its retention deadline, which
[`spinloop fleet metrics`](fleet.md#metrics) and
[`spinloop dashboard`](dashboard.md) show.

## See also

- [`spinloop dashboard`](dashboard.md) — the same facts, live and interactive
- [`spinloop fleet`](fleet.md) — driving the nodes rather than reading them
- [`spinloop remote status`](remote.md) — one environment, with its address,
  version and keep deadline in full
