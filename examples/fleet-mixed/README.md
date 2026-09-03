# A mixed fleet

One `fleet.yaml`, one set of commands, two kinds of node: machines running
`spinloop daemon` and [`spinloop remote`](../../docs/commands/remote.md)
environments. The same fan-out reaches every node, so the fleet reads as a
single table.

## How to run

### 1. Bring up a daemon (a machine node)

On the box you want in the fleet, run the daemon — it takes no Spinloop of
its own, just its flags:

```sh
SPINLOOP_API_TOKEN=… spinloop daemon
```

Put that token in a `.env` beside `fleet.yaml` (copy [`.env.example`](.env.example)).
This is the same as [`examples/fleet`](../fleet/README.md); for daemons in
containers rather than real machines, see
[`examples/fleet-docker`](../fleet-docker/README.md). What it runs is decided
by whoever starts it — see the next section.

### 2. Give each node a Spinloop source, then bring them up

`fleet.yaml`'s `file` field (or a registered `spinloop alias`, or a
same-named subdirectory) names the Spinloop that describes what a node runs
— [`gpu-box.Spinloop`](gpu-box.Spinloop) for the machine,
[`qwen.Spinloop`](qwen.Spinloop) and [`llama/Spinloop`](llama/Spinloop) for
the two environments. See
[`spinloop fleet`](../../docs/commands/fleet.md#a-nodes-spinloop-source) for
the full resolution order.

```sh
spinloop fleet start gpu-box   # tell the daemon what to run, and start it
spinloop fleet deploy --all    # create both environments from this file
```

Creating the environments this way is the same as running `spinloop remote
deploy` once per `Spinloop` that says `REMOTE <name>` — see [`spinloop
remote`](../../docs/commands/remote.md) — just one command for both.

### 3. Observe the whole fleet

```sh
spinloop fleet status        # one row per node: the machine and the environments
spinloop fleet metrics -w    # a live dashboard
spinloop fleet start qwen    # wake a sleeping environment from zero
spinloop fleet stop gpu-box  # stop the machine's engine
```

Every node renders as the same kind of row: a daemon that is down shows
`unreachable`, an environment that is not deployed yet shows `config-error`,
and the rest of the fleet still shows.

## See also

- [`examples/fleet`](../fleet/README.md) — a fleet of daemons only
- [`examples/fleet-remote`](../fleet-remote/README.md) — a fleet of remote environments only
- [`spinloop fleet`](../../docs/commands/fleet.md) — the fleet file, its node kinds, and routing
