# A mixed fleet

One `fleet.yaml`, one set of commands, two kinds of node: machines running
`spinloop daemon` and [`spinloop remote`](../../docs/commands/remote.md)
environments. The same fan-out reaches every node, so the fleet reads as a
single table.

## How to run

### 1. Bring up a daemon (a machine node)

On the box you want in the fleet, run the daemon:

```sh
SPINLOOP_API_TOKEN=… spinloop daemon ./Spinloop
```

Put that token in a `.env` beside `fleet.yaml` (copy [`.env.example`](.env.example)).
This is the same as [`examples/fleet`](../fleet/README.md); for daemons in
containers rather than real machines, see
[`examples/fleet-docker`](../fleet-docker/README.md).

### 2. Register the environments (the cloud nodes)

Each remote environment is created and registered by a `spinloop remote deploy`
of a `Spinloop` that says `REMOTE <name>` — see [`spinloop
remote`](../../docs/commands/remote.md).

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
