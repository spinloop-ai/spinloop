## Why

A machine running an agent against a fleet today needs the fleet file, every
node's bearer token, and every node's engine key. A single
OpenAI-compatible endpoint in front of the fleet removes all of it: the agent
needs only a URL and one token, and the secrets stop at the gateway instead of
being distributed to every machine. The routing half was deliberately not built
when the fleet gained client-side selection (`fleet-harness-routing`, archived
2026-08-12) because it had nothing to stand on; the selector, the engine
endpoint reporting, and the wake path it assembles are all in place now, so the
gateway is mostly assembly.

## What Changes

- New `spinloop gateway` command: a foreground process that holds a fleet file
  and serves one OpenAI-compatible endpoint for it — `/v1/models` (what a
  request can reach: what the nodes are running, and, when the fleet's wake
  allows it, what a stopped node's own source describes — the model a request
  would start it with) and reverse proxies for `/v1/chat/completions` and
  `/v1/completions`, picking a node with the existing fleet selector and
  streaming the reply through untouched.
- The gateway authenticates callers with one bearer token, the way the daemon
  does (file, environment, or command line; a non-loopback listen without one
  refuses to start), and holds each node's engine key itself, supplying it to a
  node when it wakes one — the client-side rule that the starter supplies the
  key now applies to the gateway as the starter.
- The gateway wakes a node when a request names a model nothing is serving,
  holding the request while the engine loads, bounded by a wake timeout. A
  fleet file's new top-level `wake:` setting (`on`/`off`, default `on`)
  controls this; a model no node can serve fails fast, naming each node's
  refusal.
- Pointing a launch at the gateway is the fleet file's `gateway` section (added
  by `add-fleet-harness`): a launch whose effective fleet file names a gateway
  is pointed at it, with the gateway's token taken from the client's
  environment, and `spinloop fleet route` answers such a Spinloop by saying the
  gateway has already chosen.
- Daemon status reports the name an engine serves its model under (the deploy
  config's served name) beside the model id, so a request that names an alias
  can be matched against what a node reports.
- A remote environment's status now carries what it is serving — the model id,
  and the served name beside it — read from the environment's stored deploy
  config, the same source the stats reply uses, so a gateway (or `fleet
  status`) can match a request to a running remote node the way it matches a
   local one and the fleet and remote views name it the same. Without this a
   running remote node reported its state but no model, and was invisible to
   model-based routing.
- A remote environment's status also carries where its engine answers — the
  instance's published address the control plane reports, which a daemon on the
  instance cannot know. Routing resolves it as the engine's host, so the gateway
  reaches a running remote node instead of listing its model and then refusing
  to route a request to it.
- Waking: a start refused because another client woke the node first is
  re-read, and the winner of that race is used only once its engine answers —
  not on state alone — so no caller, client or gateway, is handed an engine
  that is still loading weights.
- Selection no longer treats an engine that has not answered as serving: the
  daemon's state turns running when the engine process exists, which is before
  the weights are fetched and loaded, and during that window nothing is
  listening on the engine's port. Such a node is not selected, and a request for
  its model is held for the node to finish starting — bounded by the wake
  timeout, the engine left running on it — rather than waking a second node. An
  absent readiness reading still routes, so older daemons are unaffected.
- New standalone example `examples/gateway-docker/`: a fleet plus a gateway in
  containers, with a client that holds nothing but a URL and one token.

## Capabilities

### New Capabilities

- `fleet-gateway`: the `spinloop gateway` command — its OpenAI-compatible
  surface, caller authentication, node selection and proxying, wake behaviour
  and its timeout, and what it deliberately is not.

### Modified Capabilities

- `fleet-routing`: a launch whose effective fleet file names a gateway is
  routed to that gateway (base URL and key resolution); `fleet route` answers
  such a Spinloop; waking's race rule now requires the engine to answer before a
  raced node is used; choosing no longer treats a node whose engine has not
  answered as running what is wanted, and a pinned node still starting fails
  saying so.
- `fleet-config`: a fleet file MAY declare a top-level `wake` setting
  (`on`/`off`) deciding whether routing starts an engine on a node that is not
  running one.
- `daemon-api`: status reports the served name of the model an engine runs,
  beside the model id it already reports.
- `remote-node`: a running environment's status carries what it is serving
  (model id and served name), read from the same stored deploy config the
  stats reply reads, so the fleet and remote views name it the same.

## Impact

- `cmd/spinloop`: new `gateway.go` command; the launch path's route step and
  `fleet route` gain the endpoint branch; completion and help cover the new
  command.
- `internal/gateway` (new): the HTTP surface — auth, model listing (what runs,
  plus what a wake can start, each node's source resolved at most once in a
  short window), the reverse proxy, the in-flight wake joining, and a short
  cache over the status fan-out so a burst of requests does not fan out per
  request.
- `internal/fleet`: the wake race winner waits for the engine to answer; the
  "usable now" test (running, matching, ready) is shared with the gateway;
  selection skips a node whose daemon reports the engine not ready, and a
  caller holding the no-node-serving failure can tell that from a node already
  loading the wanted model and wait for it; the fleet file parses `wake:`.
- `internal/daemon`: status gains the served-name field, set from the stored
  deploy config, and `EngineEndpoint` gains the host a node reports when it
  knows its client-facing address (a remote environment does; a daemon never
  will); `docs/openapi.yaml` updated to match (checked by `openapi_test.go`).
- `internal/fleet`: the remote node's status mapping carries the serving facts
  the control plane now relays and the engine's published address as the
  endpoint's host, and routing resolves a reported engine host in place of the
  fleet file's; `internal/remote`: `Response` gains the served name.
- `remote/` (control plane): the start Lambda's status branch reads the
  environment's stored deploy config for the serving facts (runner, model id,
  served name) and the daemon for its activity. This changes what a deployed
  start Lambda answers, so a control-plane redeploy is needed for live
  environments to report a model; an environment deployed before the served-name
  feature reports its model id but no served name until it is redeployed.
- `docs/`: command reference for `spinloop gateway`;
  `examples/gateway-docker/` is the runnable end-to-end demonstration.
- No daemon endpoint changes, no new dependencies, no Spinloop keyword, no
  change to the launch path's ordering (route before apply). A fleet with no
  `wake:` and a Spinloop naming no gateway behave exactly as they do now.
