# A fleet of remote environments

`spinloop fleet` observes [`spinloop remote`](../../docs/commands/remote.md)
environments the same way it observes machines running `spinloop daemon`: each
environment becomes a node. There are no bearer tokens here — the control plane
signs each call with your AWS credentials — and the file names environments,
never an account, so it is safe to keep under version control.

## How to run

### 1. Have some environments

An environment is created and registered when you deploy into it: a `Spinloop`
that says `REMOTE <name>`, run through `spinloop remote deploy`, writes that
environment's control URLs to `~/.config/spinloop/remotes/<name>/remote.json`.
Deploying needs the shared control plane once before it — see [`spinloop
remote`](../../docs/commands/remote.md). List what you already have:

```sh
spinloop remote ls
```

Or create both of this fleet's environments straight from the file:

```sh
spinloop fleet deploy --all --dry-run   # see the plan for qwen and llama first
spinloop fleet deploy --all             # then create them
```

That reads [`qwen.Spinloop`](qwen.Spinloop) and
[`llama/Spinloop`](llama/Spinloop) — see the next section for how a node finds
its own Spinloop file.

### 2. Name them as a fleet

[fleet.yaml](fleet.yaml) lists the environments — the node's name is the environment:

```yaml
nodes:
  - name: qwen        # the registered environment, and what you type at `fleet start qwen`
    kind: remote
    file: ./qwen.Spinloop   # what fleet deploy reads to create it

  - name: llama
    kind: remote            # no file: resolved from llama/Spinloop instead — see fleet.yaml
```

`file` is optional — a node's own name doubles as a lookup key. `qwen` names
its Spinloop explicitly; `llama` relies on the subdirectory convention
(`llama/Spinloop` beside this file) instead, and a registered `spinloop
alias` named after a node is a third way, tried before the subdirectory. See
[`spinloop fleet`](../../docs/commands/fleet.md#a-nodes-spinloop-source) for
the full resolution order.

### 3. Observe from anywhere

From any machine your AWS credentials reach:

```sh
spinloop fleet status        # one row per environment
spinloop fleet metrics -w    # a live dashboard
spinloop fleet start qwen    # wake a sleeping environment from zero
spinloop fleet stop qwen     # scale it back down
```

An environment that has not been deployed yet shows as `config-error` on its
row, and one that is down shows as `unreachable` — either way, the rest of the
fleet still renders.

## See also

- [`spinloop fleet`](../../docs/commands/fleet.md) — the fleet file, its node kinds, and routing
- [`spinloop remote`](../../docs/commands/remote.md) — the environments these nodes drive
