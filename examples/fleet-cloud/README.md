# A fleet of cloud nodes

`spinloop fleet` observes [`spinloop cloud`](../../docs/commands/cloud.md)
nodes the same way it observes machines running `spinloop daemon`: each
becomes a fleet node. There are no bearer tokens here — the control plane
signs each call with your AWS credentials — and the file names nodes,
never an account, so it is safe to keep under version control.

## How to run

### 1. Have some cloud nodes

A cloud node is created on the control plane when you deploy it: `spinloop
cloud deploy --node <name>`, run in a directory whose `Spinloop` describes
what the node serves, deploys the node there and records it on
the plane — the same record every machine with credentials for the plane
lists, so there is nothing to register per machine.
Deploying needs the shared control plane once before it — see [`spinloop
cloud`](../../docs/commands/cloud.md). List what you already have:

```sh
spinloop cloud ls
```

Or create both of this fleet's nodes straight from the file:

```sh
spinloop fleet deploy --all --dry-run   # see the plan for qwen and llama first
spinloop fleet deploy --all             # then create them
```

That reads [`qwen/Spinloop`](qwen/Spinloop) and
[`llama/Spinloop`](llama/Spinloop) — see the next section for how a node finds
its own Spinloop file.

### 2. Name them as a fleet

[fleet.yaml](fleet.yaml) lists the nodes — each name is a node on the plane:

```yaml
nodes:
  - name: qwen        # the node on the plane, and what you type at `fleet start qwen`
    kind: cloud       # resolved from qwen/Spinloop — see fleet.yaml

  - name: llama
    kind: cloud        # resolved from llama/Spinloop the same way
```

Neither node declares a `file` field: each one's own name is already a
subdirectory beside this file (`qwen/Spinloop`, `llama/Spinloop`), so there is
nothing more to declare. A registered `spinloop alias` named after a node
would resolve the same way, and win over the subdirectory if both existed —
or a `file` field can point anywhere else entirely, which is what
[`examples/fleet-docker`](../fleet-docker/) uses to reuse one Spinloop
(`client/Spinloop`) whose name matches neither node that runs it. See
[the fleet file](../../docs/fleet-file.md#a-nodes-spinloop-source) for
the full resolution order.

### 3. Observe from anywhere

From any machine your AWS credentials reach:

```sh
spinloop status        # one row per node
spinloop metrics -w    # a live dashboard
spinloop fleet start qwen    # wake a sleeping node from zero
spinloop fleet stop qwen     # scale it back down
```

A node that has not been deployed yet shows as `config-error` on its
row, and one that is down shows as `unreachable` — either way, the rest of the
fleet still renders.

## See also

- [`spinloop fleet`](../../docs/commands/fleet.md) — the commands over these nodes
- [The fleet file](../../docs/fleet-file.md) — the file's format, node kinds, and routing
- [`spinloop cloud`](../../docs/commands/cloud.md) — the cloud nodes behind this fleet
