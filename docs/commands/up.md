# spinloop up

Start the engine for what is in the current directory — one word for the two
ways an engine gets started:

```sh
spinloop up             # a fleet.yaml here: every node; otherwise ./Spinloop's server
spinloop up gpu-box     # a fleet.yaml here: just that node
spinloop up ./Spinloop  # no fleet.yaml: serve that file
```

## The directory decides

| What is here | `up` runs |
| ------------ | --------- |
| A `fleet.yaml` | [`spinloop fleet start`](fleet.md) — every node, or the ones named |
| A resolvable `Spinloop`, no fleet file | [`spinloop serve`](serve.md) — same resolution, same output |

A `fleet.yaml` wins when both are present: a fleet directory is a fleet.

The branches are the real commands, not copies of them:

- In a fleet directory, a bare `up` starts **every** node — the `--all` form,
  because a bare `fleet start` refuses to guess. `up <node>` starts the named
  node(s); an unknown name fails the way `fleet start` does.
- Without a fleet file, `up` resolves its Spinloop exactly as `serve` does —
  a path, a registered [`alias`](alias.md), `SPINLOOP_ALIAS`, then
  `./Spinloop` — prints the command, and runs it. A directory with nothing to
  resolve fails with serve's own error.

`up` takes no flags; for the full options of either branch, use
[`spinloop fleet start`](fleet.md) or [`spinloop serve`](serve.md) directly.

## See also

- [`spinloop serve`](serve.md) — the local engine, in full
- [`spinloop fleet`](fleet.md) — the fleet, in full
