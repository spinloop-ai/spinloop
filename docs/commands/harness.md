# spinloop harness

Launch your coding agent — the **harness** — and manage which one is active.
opencode is the default; Pi is also supported. The harness is chosen at
runtime, never baked into a `Spinloop` file, so the same selection works for
either.

```sh
spinloop harness             # launch the active harness (forwards trailing args)
spinloop harness -H pi       # launch a specific harness, ignoring the default
spinloop harness --set pi    # make Pi the default for future commands
spinloop harness --get       # show the current default
```

## Which harness wins

Every `spinloop` command resolves the harness the same way:

1. `--harness`/`-H` flag
2. `SPINLOOP_HARNESS` environment variable
3. Your stored default (`spinloop harness --set`)
4. opencode

## Configure, then launch

`--spinloop`/`-O` applies an [`Spinloop`](../spinloop-file.md) on the way in — the
same work [`spinloop apply`](apply.md) does — so one command configures the agent
and launches it:

```sh
spinloop harness -O                                  # apply ./Spinloop, then launch
spinloop harness --spinloop=path/to/Spinloop             # ...or a specific one
spinloop harness --spinloop=path/to/dir                # ...or a directory holding a Spinloop
spinloop harness --spinloop=https://example.com/Spinloop # ...or a URL, fetched instead of read
```

Given bare, `--spinloop` defaults to `./Spinloop` like `apply` does; when you name
a path, attach it to the flag, because anything positional is forwarded to the
agent (`spinloop harness -O run --model x` passes `run --model x` on). The one
exception is a *leading* argument that names a Spinloop — a path, a directory
holding one, or a [registered alias](alias.md) — which is applied rather than
forwarded:

```sh
spinloop harness qwen3.6-27b                 # apply the aliased Spinloop, launch
spinloop harness qwen3.6-27b -- --agent-arg  # ...forwarding --agent-arg
spinloop harness -- qwen3.6-27b              # leading -- opts out: forward it
```

`SPINLOOP_ALIAS` decides what "the default Spinloop" means, so `spinloop harness -O`
applies the alias it names. A bare `spinloop harness` still applies nothing: the
variable chooses which Spinloop, never whether you are configured. See
[`spinloop alias`](alias.md#naming-one-for-the-whole-shell).

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-H`, `--harness` | Which harness to launch (or set `SPINLOOP_HARNESS`) |
| `-O`, `--spinloop` | Apply this Spinloop before launching (bare: `./Spinloop`) |
| `--set` | Store the default harness and exit |
| `--get` | Print the active harness instead of launching |
| `--providers` | Path to a custom catalogue, for the applied Spinloop |
| `-f`, `--fleet` | Route through this fleet file (overrides the Spinloop's `FLEET`) |
| `--node` | Pin the launch to one fleet node |
| `--prefer` | Rank fleet nodes by `idle` or `active` (overrides the fleet file) |
| `--no-wake` | Fail rather than starting an engine on an idle fleet node |
| `--wake-timeout` | How long to wait for a woken node's engine (default 5m) |

## Launching against your fleet

A Spinloop with a [`FLEET`](../spinloop-file.md#running-the-model-on-another-machine-you-own)
instruction sends the agent to a machine on your network instead of a local
engine:

```sh
spinloop harness my-spinloop          # picks a node, launches the agent at it
spinloop harness --node gpu-box my-spinloop
spinloop harness --prefer active my-spinloop
```

spinloop queries the fleet, prefers a node already serving the Spinloop's model,
and points the launched agent at that node's engine — the same injection that
carries a [`REMOTE`](remote.md) endpoint's address and key, with a selection
step in front. It reports which node it chose, and why, before the agent
starts.

When nothing is serving that model, spinloop picks a node that is not running,
tells it what to serve, starts it, and waits for its engine to answer. A node
that is already running is never stopped to make room — someone else may be
using it — so a fleet with every machine busy on other models fails rather than
displacing anyone. `--no-wake` turns starting off entirely.

Which node wins among several that could all serve you is a
[`prefer` setting](fleet.md#spreading-or-consolidating): `idle` (the default)
takes the machine that has been quiet longest, keeping a second agent off an
engine that is mid-request; `active` consolidates onto the busy one instead.

## Notes

- Trailing arguments and stdio go to the agent untouched, and its exit code is
  yours.
- Not every provider maps to every harness — [`spinloop list`](list.md) shows
  which harnesses each supports.

## See also

- [`spinloop show`](show.md) — what the active harness has configured
- [`spinloop apply`](apply.md) — configure without launching
- [`spinloop fleet route`](fleet.md#which-node-would-i-get) — which node a launch would pick
- [`examples/fleet-local/`](../../examples/fleet-local/) — routing at a single local node, end to end
