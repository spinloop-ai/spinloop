# Run a daemon node

[`spinloop serve`](local-serving.md) runs an engine in front of you until one
of you exits. `spinloop daemon` is the long-lived form: it supervises one
engine over the [HTTP control API](../http-api.md), so anything that can reach
the machine — a fleet, a gateway, a laptop across the network — can start it,
stop it, and watch it.

A daemon is what a **node** is: a machine on your network that serves engines
on request. A fleet is daemons, named.

## Run it

```sh
spinloop daemon                      # listens on :4242, binds all interfaces
spinloop daemon --loopback           # 127.0.0.1:4242; needs no token
```

The daemon stays in the foreground itself — background it with tmux, systemd,
launchd, or a container. It writes the engine's output to `daemon/engine.log`
under [spinloop's config directory](../env-vars.md#config-directory-resolution)
and tracks the engine's state: `idle`, `running`, `stopped`, `crashed`. A crash
is reported, never auto-restarted.

## Authenticate it

Requests carry `Authorization: Bearer <token>`. The token comes from one of
three places — giving two at once is an error rather than a silent precedence:

| Source | Notes |
| ------ | ----- |
| `--api-token-file <path>` | The file's contents, trimmed. Use this from a service manager. |
| `SPINLOOP_API_TOKEN` | The environment. |
| `--api-token <value>` | The token itself — readable in `ps` by every local user. |

A non-loopback listen with no token **refuses to start**; a loopback one needs
none. A literal in a systemd unit file or plist is a secret in a config file
*and* in the process list, which is why the file form exists.

## What a node holds

Nothing but the daemon. A node reads no `Spinloop`, no preset, and no
`fleet.yaml` — passing a Spinloop path to it is an error rather than being
quietly ignored. **The client that asks decides what runs**: a start request's
own deploy config, or the one stored from a previous ask via
`PUT /v1/deploy-config`. With neither, a start says so.

That is why a node and a client want different files. A client's Spinloop names
a model and a fleet; a node holds nothing. See
[`examples/fleet-local/`](https://github.com/spinloop-ai/spinloop/tree/main/examples/fleet-local)
for the whole shape on one machine.

## The API

| Endpoint | Meaning |
| -------- | ------- |
| `GET /v1/status` | Engine state, what is served, the log path, how long it has been idle |
| `POST /v1/start` | Start the engine (optional deploy-config body; 409 while one runs) |
| `POST /v1/stop` | Stop the engine (idempotent; never ends the daemon) |
| `GET /v1/metrics` | Engine token counters plus host GPU/CPU/RAM |
| `GET /v1/logs` | A slice of the engine's output, by offset |
| `PUT /v1/deploy-config` | Set what the *next* start serves |

The [full contract is `openapi.yaml`](../openapi.yaml); the
[HTTP control API page](../http-api.md) walks the endpoints in detail.

## Gating the engine

An engine can require its own key, separately from the token above — one
authorises driving the node, the other authorises using its engine. The
**caller** supplies it, in the start request; a node sources no key of its
own. The daemon writes it to a private file and points the engine at that
path, so the key never appears in the node's process list. `/v1/status`
reports only *that* a key is required, never what it is.

## Logging

Set `--log-level warn` on a node a fleet polls: a `spinloop status` refresh
every few seconds is a request each, and at that level polling is quiet while
a wrong token still shows up. Records go to stderr, so a service manager's journal or
log files keep them.

## Where next

- [`spinloop serve`](../commands/serve.md) — the foreground form, engine by engine
- [The HTTP control API](../http-api.md) — the endpoints in detail
- [Run a fleet](fleet.md) — daemons, named, from one place
- [`spinloop up`](../commands/up.md) — the one-word start for a directory that holds a Spinloop
