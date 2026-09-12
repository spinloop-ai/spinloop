# spinloop gateway

Serve a [fleet](fleet.md) under one OpenAI-compatible endpoint. The gateway is
the fleet client wearing a server: it holds a `fleet.yaml`, answers
`/v1/models` and completion requests by choosing a node with the fleet's own
selector, and wakes a node when nothing is serving what a request asks for — so
a machine running agents needs nothing but a URL and one token.

```sh
spinloop gateway               # serves ./fleet.yaml on :4000
spinloop gateway --fleet ./fleet.yaml
spinloop gateway --listen 0.0.0.0:4000
spinloop gateway --api-token-file /run/secrets/gateway
```

It runs in the foreground, the way [`spinloop serve`](serve.md) does: it holds
the fleet file it serves, and a signal shuts it down cleanly. On startup it
resolves the fleet file and its own token, and checks the token references the
file names — a `tokenEnv` or `engineTokenEnv` variable set nowhere fails here,
naming the node, rather than surfacing later as a per-request authentication
failure. It prints the address to name in the fleet file's `gateway` section:

```
Gateway for fleet.yaml is listening on [::]:4000
Name http://<this-machine>:4000 in the fleet file's gateway section
```

The host it can know is the one it was told to bind; for a wildcard bind the
machine's own name is only the operator's to know, so the printed address says
`<this-machine>` and you fill in whatever this machine is called from the other
side.

## Pointing an agent at it

A [`gateway` section](fleet.md#gateway) in the fleet file names the address and
the variable holding the token. A launch routed through that file — `spinloop
harness -f` or `spinloop fleet harness` — is pointed at the gateway rather than
a node: the section's address is the agent's base URL (with the OpenAI-
compatible `/v1` prefix added when it carries no path), and the agent
authenticates with the gateway's token, resolved the way a key is resolved
elsewhere: an `ENV` instruction, then the process environment, then the `.env`
beside the Spinloop. `OPENAI_API_KEY` stands in where the section names no
variable. An agent pointed at the gateway holds exactly that one credential;
the node tokens and engine keys live with the gateway, which presents them to
the nodes and the engines:

```yaml
# fleet.yaml
gateway:
  url: http://gateway.internal:4000
  tokenEnv: GATEWAY_TOKEN
```

```sh
spinloop fleet harness -O=./Spinloop   # from the fleet file's directory
```

A Spinloop beside that file then needs only the model, and the address travels
with the file when the gateway moves. See
[The `Spinloop` file](../spinloop-file.md#running-the-model-on-another-machine-you-own).

## What it answers

| Path | Meaning |
| ---- | ------- |
| `GET /health` | That the gateway is up. It touches no node on purpose — it is how you tell the gateway down from the fleet down. |
| `GET /v1/models` | The OpenAI list of what a request can reach: what the running nodes report (the served name when a node reports one, else the model id), and — when [wake](#waking-a-node) is on — what a stopped node's own source describes, the model a request would start it with. Duplicates once. Nothing reachable is an empty list, not an error. |
| `POST /v1/chat/completions` | Routed to the node serving the request's `model`, the way a launch routes. |
| `POST /v1/completions` | The same, for the completions endpoint. |

A request naming no `model` is refused saying so, and a path the gateway does
not serve is refused with a `404` naming the ones it does.

The list is what a request can reach, so it is bounded by what the gateway can
start: a running node contributes only what it reports — a running engine is
never displaced to make room — and a `kind: remote` environment contributes
nothing beyond what it runs, because a request never wakes one. With
`wake: off`, only what is running is listed. Each node's source is read at most
once in a short window, so a poll of the models list is cheap.

### Routing a request

A completion request is answered by the fleet's own selection: a node already
running the model wins, ranked by the fleet file's `prefer` with fleet-file
order breaking ties. A node whose engine is bound to loopback without an
[`engine` override](fleet.md#where-a-nodes-engine-answers) is never selected,
and when it is the only match the failure says so rather than holding the
request until the wake timeout.

The request's body goes out unmodified and streamed replies are flushed as
they are produced, so a `stream: true` request streams through. The caller's
authorisation never travels past the gateway: the engine is reached with the
key its fleet entry names (`engineTokenEnv`, or the fleet-wide `apiKeyEnv` for
a `kind: remote` node), and an ungated engine is reached with none. The reply
the engine gives is the reply the caller gets — the gateway never retries
another node, and an upstream failure reaches the caller as an error naming
the node.

### Waking a node

When no running node serves the model and the fleet file's
[`wake`](fleet.md#waking) setting allows it, the gateway starts a node with
the config that node's own Spinloop source resolves to — only nodes whose
source describes the requested model are candidates, and a node whose stored
config already matches is tried first — and holds the request until the engine
answers, bounded by `--wake-timeout` (default 5m). A timeout fails the request
saying so and leaves the engine running, so a slow load is not thrown away.
Concurrent requests for the same model wake at most one engine: a request that
loses the start to the daemon's "already running" answer takes the node the
other one started.

With `wake: off`, or when no node's source describes the model, a request
nothing is serving fails without starting anything, naming the node and the
`spinloop fleet start <node>` command that would start it.

The gateway needs the same environment a machine running
`spinloop fleet start` would: the tokens the fleet file names, set in its
process environment or in a `.env` beside the fleet file.

## The gateway's token

Callers present the gateway's token as a bearer token on every request — a
wrong or missing one is a `401`. The token comes from one of three places, the
same rules the [daemon's](serve.md#the-control-api---api-and-spinloop-daemon)
token follows, and giving two at once is an error rather than a silent
precedence:

| Source | Notes |
| ------ | ----- |
| `--api-token-file <path>` | The file's contents, trimmed. |
| `SPINLOOP_API_TOKEN` | The environment. |
| `--api-token <value>` | The token itself — readable by every local user through `ps`, like the daemon's. |

A non-loopback listen with no token refuses to start, naming the three ways to
supply one; a loopback listen (`--loopback`, or `--listen 127.0.0.1:4000`)
needs none. The default, `:4000`, binds every interface — the shape a gateway
on a shared machine wants, and the reason the token is not optional there.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-f`, `--fleet` | The fleet file to serve (default `./fleet.yaml`) |
| `--listen` | The address to listen on (default `:4000`) |
| `-l`, `--loopback` | Bind to loopback on the default port (`127.0.0.1:4000`); needs no token |
| `--api-token-file` | Read the gateway's bearer token from this file |
| `--api-token` | The gateway's bearer token |
| `--wake-timeout` | How long to wait for a woken engine to answer (default 5m) |

## See also

- [`spinloop fleet`](fleet.md) — the file the gateway serves, and the nodes it
  drives
- [`spinloop daemon`](serve.md#the-control-api---api-and-spinloop-daemon) — what
  each node runs
 - [`examples/gateway-docker/`](../../examples/gateway-docker/) — a gateway and
   its fleet in containers, with the test suite that asserts all of this
