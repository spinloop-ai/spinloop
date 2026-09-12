# A gateway you can actually run

Two `spinloop daemon` nodes and a `spinloop gateway` in front of them, on your
laptop, in containers — so you can see what
[`spinloop gateway`](../../docs/commands/gateway.md) does before pointing a real
fleet at it. No GPUs, no cloud, no model downloads.

```sh
cp .env.example .env
docker compose up -d --build

# from this directory, with the tokens exported
set -a && . ./.env && set +a

# the gateway's own surface
curl -H "Authorization: Bearer $GATEWAY_TOKEN" http://127.0.0.1:4000/v1/models
curl -X POST -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}' \
  http://127.0.0.1:4000/v1/chat/completions

# and the fleet underneath it, the way spinloop fleet drives any fleet
spinloop fleet status --fleet ./fleet.yaml
spinloop fleet start node-b --fleet ./fleet.yaml
```

The stack brings up **two gateways** over the same two nodes: `gateway` on port
4000, which wakes a node when nothing is serving, and `gateway-cold` on port
4001, which refuses to — the `wake: off` policy from its own fleet file, so the
two ways of running a fleet are visible side by side.

## What is real and what is not

**Real**: each node runs the actual `spinloop daemon` from this repository,
serving its control API over the network with bearer-token auth, and supervises
its engine as a real child process. The gateway is the same binary running
`spinloop gateway`: it chooses a node with the fleet's own selector, wakes one
when nothing is serving, holds the request until the engine answers, and swaps
the caller's authorisation for the engine key its fleet entry names.

**Not real**: the engine. Instead of `llama-server` there is a
[`llama-server` shim](shim/llama-server) that starts
[Imposter](https://imposter.sh)'s native engine, which serves a canned
`/health`, a `/metrics` in llama.cpp's Prometheus dialect, and OpenAI-shaped
completion replies — streamed, when asked, in the server-sent-events shape. So
a request through the gateway genuinely travels to a woken node and back, and
the streamed reply you see is the one the fake engine produced. Nothing is
inferring anything.

That trade is deliberate: what is being demonstrated (and tested) is the
gateway's routing, waking and key handling, not inference.

**Also real**: the keys. Each engine is gated with the key its fleet entry
names — the node reports a key is required and never what it is, and the key
reaches the engine as a file path, so `docker compose exec node-a ps ax` shows
`--api-key-file`, not the key. The caller of the gateway presents only the
gateway's token; the node tokens and engine keys live with the gateway, which
is the one place that holds all of them.

There are three Spinloops here:

- [`client/Spinloop`](client/Spinloop) — what an *agent's* machine wears: just
  the model. Where it is served lives in [`fleet.yaml`](fleet.yaml)'s `gateway`
  section, and `spinloop fleet harness` from this directory reads it: the
  gateway has done the choosing, and the agent is only pointed at it, with the
  gateway's token as its key.
- [`node/Spinloop`](node/Spinloop) — what a *node* runs when started. Its
  `BASEURL` binds the engine to every interface, which is why the gateway — a
  different container — can reach it at all.
- the nodes hold no Spinloop of their own in the container; the image bakes a
  copy of `node/Spinloop` beside the gateway's fleet files so the gateway's own
  wakes resolve what a node runs.

## Things worth trying

```sh
# Cold request: nothing is serving, so the gateway wakes a node, holds the
# request until the engine answers, and streams the reply back.
curl -X POST -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"fake-model","stream":true,"messages":[{"role":"user","content":"hi"}]}' \
  http://127.0.0.1:4000/v1/chat/completions

# The same at the wake: off gateway: refused, naming the node and the command
# that would start it.
curl -i -X POST -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}' \
  http://127.0.0.1:4001/v1/chat/completions

# wake: off still routes to what is already running — it decides whether to
# start, not whether to answer.
spinloop fleet start node-a --fleet ./fleet.yaml
curl -X POST -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}' \
  http://127.0.0.1:4001/v1/chat/completions

# A wrong token is a 401, not a routing decision.
curl -i http://127.0.0.1:4000/v1/models

# The engine, directly: gated, like any engine the fleet gates.
curl -i -X POST http://127.0.0.1:18080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}'

# Launch an agent at the fleet's gateway: fleet.yaml's gateway section points
# the agent at the gateway, with the gateway's token, and nothing else.
spinloop fleet harness -O=./client/Spinloop
```

## It is also the integration test

`./run-tests.sh` drives this same stack and asserts the behaviours above: a
model is listed once running, a cold request wakes a node and streams a reply,
the engine key is injected and never reaches the caller, a wrong token is 401,
and `wake: off` refuses. CI runs it on every pull request, which is the point:
an example that is exercised cannot quietly stop working.

```sh
./run-tests.sh          # up, assert, tear down
./run-tests.sh --keep   # leave the stack running to poke at
```

## How it fits together

| File | What it is |
| --- | --- |
| `compose.yaml` | Two nodes, a gateway that wakes, a gateway that refuses to. Every service that listens on a non-loopback address needs a token, and the gateways need the fleet file's node tokens and engine keys in their environment. |
| `fleet.yaml` | The *operator's* view: the two nodes over their published ports, with `engine:` blocks because the engines are published on ports the daemons cannot know — and a `gateway` section naming the waking gateway, so a launch through this file is pointed at it. |
| `gateway/fleet.yaml` | What the `gateway` service serves: the same two nodes, addressed by compose service name — where the gateway can reach them, with no `engine:` override needed. |
| `gateway/fleet-cold.yaml` | The same fleet with `wake: off`, served by `gateway-cold`. |
| `Dockerfile` | Builds spinloop from this working tree, adds the Imposter engine and the shim, and bakes the gateway's files in. |
| `shim/llama-server` | Stands in for the engine binary. Reads the key file the daemon passes and hands the mock its gate as an environment variable, so the value never rides on a command line. |
| `engine/` | What the fake engine serves: `/health` and `/metrics` for the daemon, and the gated, stream-answering OpenAI routes. |
| `node/Spinloop` | What a node runs when started: a model, and a `BASEURL` that binds the engine to every interface. |
| `client/Spinloop` | What an *agent's* machine wears: the model. Its address comes from `fleet.yaml`'s gateway section, via `spinloop fleet harness`. |

Two details that are easy to get wrong, and matter:

- **The node's `BASEURL` binds the engine wide.** Without it llama-server
  binds `127.0.0.1`, the daemon reports the engine loopback-only, and the
  gateway is right to refuse routing to it — an engine that answers only on
  its own machine is not a candidate for a gateway on another.
- **The shim execs the engine binary, not `imposter up`.** The CLI wrapper
  exits 0 when its child dies, which the daemon would correctly record as a
  clean stop — so a crash test would pass while testing nothing.

## See also

- [`examples/fleet-docker/`](../fleet-docker/) — a plain fleet, no gateway
- [`docs/commands/gateway.md`](../../docs/commands/gateway.md)
- [`docs/commands/fleet.md`](../../docs/commands/fleet.md#the-gateway-section) — the `gateway` section
- [HTTP Control API](../../docs/http-api.md)
