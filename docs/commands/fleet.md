# spinloop fleet

Observe and drive every engine you run, from one place. Each machine runs
[`spinloop daemon`](serve.md#the-control-api---api-and-spinloop-daemon); a
`fleet.yaml` names them, and `spinloop fleet` fans out over their control APIs.

```sh
spinloop fleet status          # one row per node: state and what it serves
spinloop fleet metrics         # each node's engine + system metrics
spinloop fleet metrics -w      # the same, redrawn in place until interrupted
spinloop fleet dashboard       # the interactive tiled view — watch it, drive it
spinloop fleet route my-spinloop # which node a harness launch would pick
spinloop fleet start gpu-box   # start one or more nodes' engines
spinloop fleet start --all     # start every node in the fleet
spinloop fleet stop gpu-box    # stop one or more nodes' engines
spinloop fleet deploy --all    # create every kind: remote node's AWS environment
```

A fleet is also where [`spinloop harness`](harness.md#launching-against-your-fleet)
sends an agent: a Spinloop naming a `FLEET` picks a node and launches against it,
so the machine you are sitting at needs no engine of its own.

## Try it without any hardware

[`examples/fleet-docker/`](../../examples/fleet-docker/) brings up a real
three-node fleet in containers — real daemons, real auth, a fake engine — so
you can see all of this working before setting up a single machine:

```sh
cd examples/fleet-docker && cp .env.example .env
docker compose up -d --build
set -a && . ./.env && set +a
spinloop fleet status --fleet ./fleet.yaml
```

## `fleet.yaml`

A list of nodes and how to reach each one. It holds **no secrets** — a node
that needs a bearer token names the environment variable holding it:

```yaml
nodes:
  - name: studio          # what you type at `fleet start <node>`
    host: studio.local    # LAN name, tailscale name, or an address

  - name: gpu-box
    host: 198.51.100.7    # a tailscale address, say
    port: 4242            # optional; the daemon's default when omitted
    tokenEnv: GPU_BOX_TOKEN   # the *name* of the variable, never the token
```

The file is found the way a `Spinloop` is: `./fleet.yaml` in the working
directory, or `--fleet <path>`.

### Where a node's engine answers

A node's `host` and `port` name its **daemon**, which is a different port from
the **engine** it supervises. For [routing](#which-node-would-i-get) spinloop
needs the engine's, and the daemon reports it — so most nodes need nothing
more. Declare an `engine` block for the cases a daemon cannot describe:

```yaml
nodes:
  - name: containerised
    host: docker-host
    engine:
      port: 18080          # published port, not the one it binds inside

  - name: proxied
    host: node.local
    engine:
      host: https://engine.example   # a reverse proxy in front of the engine
      path: /openai                  # when it is not the usual /v1
```

Each field falls back independently to what spinloop would otherwise derive: the
node's own `host`, and the port and path the daemon reports.

An engine bound to loopback answers only on its own machine. Routing to it from
elsewhere fails with that explanation rather than a bare connection refused —
bind the engine to a reachable address (llama.cpp's `--host 0.0.0.0`), or
declare an `engine` block, which is you taking responsibility for reachability.

### Remote environments

`kind` (defaulted to `daemon`) says how the fleet reaches a node. A node can
also be an [`spinloop remote`](remote.md) environment rather than a machine: its
`name` is the registered environment it drives — no `host` needed — and it is
reached through its control plane, which signs each call with your AWS
credentials, so it needs no bearer token:

```yaml
nodes:
  - name: qwen          # the registered environment, and what you type at `fleet start <node>`
    kind: remote
```

The environment's control URLs live in its `remote.json` (under
`~/.config/spinloop/remotes/<name>/`), written by `spinloop remote deploy` — or by
[`spinloop fleet deploy`](#deploying-remote-nodes), which creates it from the
fleet file itself — and never stored in the fleet file. So a daemon and an
environment sit side by side as the same kind of row, and an environment that
has not been deployed yet shows as `config-error` on its row rather than
blanking the fleet. See
[`examples/fleet-remote`](../../examples/fleet-remote/README.md) and
[`examples/fleet-mixed`](../../examples/fleet-mixed/README.md).

### A node's Spinloop source

Both `fleet deploy` (for a `kind: remote` node's environment) and `fleet
start` (for a `kind: daemon` node's engine) need to know what Spinloop file
describes what a node runs. A node names it with `file`, resolved relative to
the fleet file:

```yaml
nodes:
  - name: qwen
    kind: remote
    file: ./envs/qwen.Spinloop
```

`file` is optional, because the node's own `name` already doubles as a lookup
key. When it is absent, resolution tries, in order:

1. `name` registered as a `spinloop alias` (`spinloop alias add qwen
   ./envs/qwen.Spinloop`) — the same lookup a bare `spinloop remote deploy
   qwen` already performs;
2. a subdirectory named after the node, beside the fleet file — `qwen/Spinloop`
   next to `fleet.yaml` for a node named `qwen`, no fields needed on either
   side.

A fleet laid out as one subdirectory per node therefore needs nothing beyond
each node's own `name`:

```
fleet.yaml
qwen/Spinloop
llama/Spinloop
```

Nothing resolving is a per-node error naming all three ways a source could
have been given. For `fleet deploy` that always fails the node (there is
nothing to create an environment from); for `fleet start` on a `kind: daemon`
node it likewise fails that node's start — there is no fallback to a plain,
config-less start once this field exists. A `kind: remote` node's `start` is
unaffected by any of this: what it serves is fixed at deploy time, not pushed
at start time.

This does not apply to `spinloop fleet dashboard`'s `s` key, which still
starts the selected node with a plain start, whatever the CLI's `fleet start`
would resolve for it.

### Spreading or consolidating

`prefer` decides which node wins when several could all serve you:

```yaml
prefer: idle      # or: active
nodes: …
```

- **`idle`** (the default) — the machine quiet longest wins. A node that is
  mid-request is the *least* idle of all, so it is the last one chosen. Use it
  when several people share the fleet, or you run several agents at once.
- **`active`** — the most recently active wins, consolidating sessions onto one
  engine and leaving the others free to be woken for another model, or left
  asleep.

`spinloop harness --prefer <value>` and `spinloop fleet route --prefer <value>`
override the file for one command, which is the cheap way to see what the other
setting would do before committing to it.

### Tokens

`tokenEnv` names an environment variable; the value is resolved from the
process environment first, then a `.env` beside the `fleet.yaml` — the same
precedence spinloop uses everywhere, so an exported value wins and the `.env`
only fills a gap. Put the secrets there:

```sh
# .env beside fleet.yaml (gitignored)
GPU_BOX_TOKEN=…
```

A node with no `tokenEnv` is contacted without authentication, which is
correct for a daemon bound to loopback. Any node reachable over the network
needs a token — the daemon refuses to listen on a non-loopback address without
one.

A `tokenEnv` naming a variable that is set nowhere is reported against that
node as `config-error`, so a typo shows up on its row rather than as a
mysterious `unauthorized`.

A node whose **engine** needs a key names that separately, with
`engineTokenEnv`. The two are different credentials — one authorises driving the
node, the other authorises using its engine — and a node may need either, both,
or neither. It is resolved exactly as `tokenEnv` is, and the daemon never hands
its engine's key out: it says only that one is required.

```yaml
  - name: gated
    host: gated.local
    tokenEnv: GATED_TOKEN             # to drive the daemon
    engineTokenEnv: GATED_ENGINE_KEY  # to talk to its engine
```

A `kind: remote` environment is always keyed, so it needs an engine key too —
its `engineTokenEnv` works as above, and a fleet-wide `apiKeyEnv` is the
default for every remote node that does not name one of its own. Either way the
launch fails before it starts the agent rather than pointing it at a gate it
cannot pass:

```yaml
apiKeyEnv: REMOTE_ENGINE_KEY   # the default for every kind: remote node
nodes:
  - name: qwen
    kind: remote
    engineTokenEnv: OTHER_KEY  # overrides it for this node
```

## A node that is down never blanks the view

Fan-out is for observing, so a node that cannot be reached is a **row**, not a
failure — the rest of the fleet still renders and the command still exits 0:

```
NODE     STATE         SERVING
studio   running       llamacpp  org/qwen  (up 1h 2m 5s)  (last active 12s ago)
gpu-box  idle          llamacpp  org/qwen
offline  unreachable   dial tcp 10.0.0.9:4242: connect: connection refused
```

"last active" comes from the activity each daemon tracks, so a glance answers
"which of my nodes is doing nothing?". It is absent until a node's engine has
actually done some work — a daemon that has served nothing reports no activity
rather than claiming it has been quiet since it started. The wording avoids
"idle" deliberately: that word is already an engine *state*, meaning nothing
has been started at all.

| Outcome | Meaning |
| --- | --- |
| *(a state)* | The node answered: `idle`, `running`, `stopped`, `crashed` |
| `unreachable` | No answer at all — refused, timed out, no such host |
| `unauthorized` | The box is up; the token was rejected |
| `config-error` | The node could not be called — usually a `tokenEnv` that resolves to nothing |
| `failed` | The daemon answered with an error — the node is fine, the request was refused |

## Metrics

`spinloop fleet metrics` renders each node's engine and system metrics in the
same `bar` (default), `table`, and `json` formats as
[`spinloop remote metrics`](remote.md) — they share the renderers, so a node in
your fleet and a cloud endpoint look the same.

Each node's block carries the same `last active` figure the status table
shows, for the reasons given above, and on the same terms: absent until the
node's engine has done some work. A node whose engine has *stopped* still
shows it — the daemon keeps the record across a stop, and "how long since this
did anything?" is worth more about a stopped engine than about a busy one.

`--watch`/`-w` redraws the whole fleet on an interval, clearing the screen in
place with no scrollback. Each refresh is rendered into a buffer first, so a
slow node delays the refresh but never tears the display. Ctrl+C exits
cleanly.

The `json` format is labelled by node and **includes the nodes that failed**,
with their outcome and reason — so a consumer sees the whole fleet rather than
silently missing whatever was down:

```json
[
  { "node": "studio", "outcome": "ok", "metrics": { "state": "running", "…": "…" } },
  { "node": "offline", "outcome": "unreachable", "error": "dial tcp …: connection refused" }
]
```

## The dashboard

`spinloop fleet dashboard` is that same board as a live view: one tile per
node, repainted in place, each drawing exactly what `fleet metrics`' bar
format prints for the node — state and uptime, what it serves, the CPU/GPU/RAM
bars, the token counters — so the view and the one-shot command never word a
number differently. A node that is down is a tile that says why, and a node
whose token reference resolves to nothing holds that reason for the life of
the view:

```sh
spinloop fleet dashboard                # ./fleet.yaml
spinloop fleet dashboard --fleet f.yaml # another fleet file
```

| Key | Does |
| --- | ---- |
| `j`/`k` or the arrows | Move the selection, in file order (no wrap) |
| `PgUp`/`PgDn` | Page the grid when there are more nodes than fit |
| `Enter` | Open a full-screen view of the selected node |
| `r` | Force a refresh of every node, now |
| `s` | Start the selected node — without confirmation |
| `a` | Abandon a start in flight on the selected node — the wait ends, the node is free again (a stop in flight is not abortable) |
| `x` | Stop the selected node — it asks first (`y` sends, `n` or `esc` cancel) |
| `q` or `Ctrl+C` | Leave |

The board keeps its own cadence: local machines are read every two seconds,
and a [`kind: remote`](#remote-environments) environment every 60 — one
status call a minute, because its status is a signed control-plane call, not a
local socket, and a cold instance changes state on the scale of minutes. `r`
is due for every node whatever those deadlines say.

`start` runs the same node operation `fleet start` does, without
confirmation, and carries no deadline, because a cloud wake takes minutes and
the call holds for the lot. While it runs, the node's tile carries the start —
the verb and the control plane's own status lines, in place of the node's last
report — because that is the truth until the report returns. An action is one
per node, not one per board: while one node is waking, select another and
start it, and the two wakes run side by side, each reported on its own tile.
When an action finishes, its tile goes back to the node's next report and its
outcome lands on the status line at the foot of the view. A start's wait can
be abandoned: `a` ends the dashboard's wait on the node's in-flight start,
and the tile is free to start or stop again. The abort ends the wait, not
the work — a cancelled client cannot take a wake the cloud is carrying back
— so the line says the wait was *abandoned*, not that the node failed, and a
wake that was in fact completing shows up as a running node on the next
refresh. A stop in flight is not abortable: it targets an engine already
running rather than a cold wake with no deadline of its own, and `a` drives
nothing while one is in progress.

Everything else in the view is `fleet status`/`metrics`/`logs` in place — it
is read-only apart from those three action keys. It needs a real terminal: a
piped run is refused, and it says so by way of `fleet metrics --watch`, which
is the streamable surface.

### The node detail view

`Enter` on a tile opens a full-screen view of that node in place of the grid:
its metrics, unclipped to the tile's 42 columns, its engine log tailed and
followed the way `fleet logs -f` follows one node, and a footer naming the
keys the view answers to. `Esc` closes it and returns to the grid with the
same node still selected.

```sh
spinloop fleet dashboard
# select a node, press Enter for its full metrics and log, Esc to go back
```

`s`, `x` and `a` drive the node shown exactly as they drive the selected node
on the grid — the same no-confirmation start, the same stop confirmation, the
same abandon. `q`/`Ctrl+C` are grid keys only and do nothing here — `Esc` back
to the grid first, then quit from there — so a stray quit keystroke while
looking at a node can't end the session out from under you. The rest of the
fleet keeps refreshing behind the view, and any action already in flight on
another node keeps running. A node whose engine has never run shows the same
explanation `fleet logs` gives for it, not an empty pane.

`f` pauses and resumes the log's follow, independently of everything else in
the view — the metrics section keeps refreshing either way. The header names
the state (`log: following` / `log: paused`). Pausing does not lose anything:
resuming fetches whatever the engine wrote in the meantime, the same as a
poll that simply ran late.

## Logs

`spinloop fleet logs` prints what your engines actually said — the answer to the
question `fleet status` raises when it reports a node as `crashed`.

```sh
spinloop fleet logs              # the tail of every node's engine log
spinloop fleet logs gpu-box      # just that node
spinloop fleet logs -f           # follow, until you interrupt it
spinloop fleet logs --limit 500  # more backlog per node
```

Each node's daemon captures its engine's stdout and stderr to a file, and
serves a slice of it over [`GET /v1/logs`](../http-api.md). Reading is safe, so
unlike `start` and `stop` this fans out across the whole fleet by default;
naming a node narrows it to one.

With more than one node talking, every line is prefixed with the node it came
from. Reading a single node leaves the prefix off, so it reads like that node's
own log. Lines are **not** interleaved between nodes: engine output carries no
timestamp we can trust, so merging several machines' lines would invent a
chronology that isn't there. Each node's output stays in its own order.

Following resumes each node from a byte offset that node reported, so a line is
never printed twice and none is missed — no overlap window, no guessing. Nodes
are polled independently, because each log is its own file with its own
position.

Nodes with nothing to give say so rather than vanishing: one that has never run
an engine, one that is unreachable, and one whose daemon is older than the
endpoint (which names itself as needing an upgrade — a fleet mid-rollout will
legitimately hold a mix).

| Flag | Meaning |
| ---- | ------- |
| `--limit` | Lines of backlog per node (default 200) |
| `-f`, `--follow` | Keep printing new output until interrupted |
| `--format` | `text` (default) or `json` |

Two things worth knowing. Engine output can carry prompts and model output, and
it crosses the network to whoever holds the node's token — the same trust
boundary as `start` and `stop`, but the content is more revealing. And the
daemon does **not** rotate its engine log: it grows for the daemon's lifetime,
so a long-lived node accumulates. Reads are always bounded, so this costs disk
on the node rather than anything at the client.

## Which node would I get?

`spinloop fleet route` reports the node a
[harness launch](harness.md#launching-against-your-fleet) would pick for an
Spinloop, and **changes nothing** — no config pushed, no engine started, no
harness config written:

```sh
spinloop fleet route my-spinloop
spinloop fleet route --prefer active my-spinloop
```

```
Spinloop: ./my-spinloop/Spinloop
Fleet:  ./fleet.yaml
Prefer: idle

Would use gpu-box at http://gpu-box:8080/v1
  serving qwen3-27b, last active 312s ago (prefer idle)
```

When nothing is serving that model it shows the whole fleet's state and names
the node a real launch would wake, without waking it:

```
no node in ./fleet.yaml is serving qwen3-27b:
  studio           idle
  gpu-box          running  some-other-model
  laptop           unreachable (connection refused)

A launch would wake studio and wait for its engine. Nothing has been started.
```

Use it to check a route before an agent depends on it, to see what the other
`prefer` setting would choose, or to work out why a launch landed where it did.

## Starting and stopping

`fleet start` and `fleet stop` take one or more node names, or `--all` for
the whole fleet:

```sh
spinloop fleet start gpu-box           # one node
spinloop fleet start gpu-box gpu-box-2 # several
spinloop fleet start --all             # every node in the file
spinloop fleet stop --all
```

With neither a node nor `--all` they list the fleet and do nothing, rather
than acting on the whole fleet by accident; `--all` together with node names
is refused as ambiguous. An unknown name fails before anything is touched,
naming the nodes you could have meant. Several targeted nodes are driven
independently — one node's failure is reported against it alone and does not
stop the others, and the command exits non-zero if any of them failed. The
daemon's own rules still hold: starting a node whose engine is already
running reports its conflict, and stopping one that is not running succeeds
quietly.

**Starting a `kind: daemon` node now requires its [Spinloop
source](#a-nodes-spinloop-source) to resolve.** When it does, `fleet start`
derives a deploy config from it and pushes it with the start (`StartWith`) —
telling the daemon what to run, the same way a routed `harness` launch
already tells a node what to run when it wakes one. When it does not resolve,
`fleet start` fails that node rather than starting it with whatever the
daemon already happens to have configured. This is a breaking change: every
fleet file with a `kind: daemon` node needs a `file` field, a matching alias,
or a matching subdirectory added, or `fleet start` fails for that node. A
`kind: remote` node's `start` is unaffected either way.

## Deploying remote nodes

`fleet deploy` creates the AWS environment for one or more `kind: remote`
nodes — the step that otherwise has to happen outside the fleet file
entirely, one `spinloop remote deploy <file>` at a time:

```sh
spinloop fleet deploy qwen           # one node
spinloop fleet deploy qwen llama     # several
spinloop fleet deploy --all          # every kind: remote node in the file
```

Each node deploys from its own resolved [Spinloop
source](#a-nodes-spinloop-source), reusing the exact derivation, consent, and
registration `spinloop remote deploy` uses for the same file — the two can
never disagree about what a given Spinloop deploys. A `kind: daemon` node
named explicitly fails the command, explaining that `deploy` provisions cloud
environments and that node is not one; `--all` only ever selects `kind:
remote` nodes, so a daemon node is never swept in by it. As with
`start`/`stop`, no node and no `--all` lists the fleet's `kind: remote` nodes
and deploys nothing, `--all` plus node names is refused as ambiguous, and
several targeted nodes deploy independently — one node's guard or failure is
reported against it alone.

```sh
spinloop fleet deploy --all --dry-run     # print every plan, deploy nothing
spinloop fleet deploy qwen --overwrite    # redeploy over a registered environment
```

`--dry-run`, `--overwrite`, `--reseed`, `--allowed-cidr`, `--region`, and
`--spinloop-version` mean exactly what they mean on [`spinloop remote
deploy`](remote.md), applied per node.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `--fleet <path>` | The fleet file (default `./fleet.yaml`) |
| `--all` | `start`/`stop`/`deploy`: act on every node (or every `kind: remote` node, for `deploy`) instead of named ones |
| `--node <name>` | `route` only: report this node rather than choosing one |
| `--prefer` | `route` only: rank by `idle` or `active`, overriding the file |
| `--format` | `metrics`: `bar` (default), `table`, or `json`; `logs`: `text` (default) or `json` |
| `-w`, `--watch` | `metrics` only: redraw on an interval until interrupted |
| `-f`, `--follow` | `logs` only: keep printing new output until interrupted |
| `--limit` | `logs` only: lines of backlog per node (default 200) |
| `-n`, `--dry-run` | `deploy` only: print the plan for each targeted node without deploying |
| `--overwrite` | `deploy` only: proceed against an already-registered or live environment |
| `--reseed` | `deploy` only: re-fetch the weights even if already in S3 |
| `--allowed-cidr` | `deploy` only: who may reach each environment's instance |
| `--region` | `deploy` only: AWS region of the control plane |
| `--spinloop-version` | `deploy` only: spinloop release each environment installs at boot |

## See also

- [`examples/fleet-local/`](../../examples/fleet-local/) — a fleet of one, on your own machine
- [`examples/fleet-docker/`](../../examples/fleet-docker/) — a runnable fleet
- [`spinloop daemon`](serve.md) — what runs on each node
- [HTTP Control API](../http-api.md) — the API the fleet client speaks
- [Environment variables](../env-vars.md)
