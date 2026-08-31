# Fleet

Observing several machines' engines from one place.

Each machine runs the daemon:

```sh
# on studio.local and gpu-box, with a token since they are network-reachable
SPINLOOP_API_TOKEN=… spinloop daemon ./Spinloop

# on this machine, loopback-only needs no token
spinloop daemon --api-addr 127.0.0.1:4242 ./Spinloop
```

Then from anywhere that can reach them:

```sh
cp .env.example .env    # fill in each node's token
spinloop fleet status
spinloop fleet metrics -w
spinloop fleet start gpu-box
```

A node that is down shows as `unreachable` and the rest of the fleet still
renders. See [`docs/commands/fleet.md`](../../docs/commands/fleet.md).

## No machines to hand?

[`examples/fleet-docker/`](../fleet-docker/) brings up a real multi-node fleet
in containers — real daemons with a fake engine — so you can try `spinloop fleet`
on a laptop before setting any of this up for real.
