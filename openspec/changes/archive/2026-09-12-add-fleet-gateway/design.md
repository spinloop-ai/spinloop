## Context

See proposal.md — Why. What matters for the approach is what already exists:

- `internal/fleet` has the whole routing half: `Select` (fan-out plus the pure
  `rank`), `Wake` (start-with-config, the warm-first candidate ordering, the
  already-running race, `waitReady`), `EngineBaseURL` (composing the engine
  address from the fleet file and the node's report), and `engineKeyFor`
  (resolving a running engine's key). A gateway is a consumer of all of it.
- The wake path's one gap: when a start loses to another client's
  already-running 409, `Wake` re-reads status and takes the node on the state
  alone — a node can report `running` while still loading weights. Fine for a
  client-side launch (the agent starts and the engine comes up moments later);
  a gateway holding a caller's request must not proxy to an engine that is not
  answering.
- The daemon's status says where the engine serves (`EngineEndpoint`) and, on
  current daemons, whether it has answered its own health check (`Ready`), but
  it reports the model id only, not the served name: a node started under an
  alias answers requests for the alias and reports the id, so a router seeing
  only status cannot match a request that carries the alias.
- The launch path consumes a `fleet.Choice`: `applyBeforeLaunch` writes
  `choice.BaseURL` into the provider slot a `REMOTE` fills and wires
  `choice.APIKey` through the same resolver the remote path uses. The endpoint
  branch of `FLEET` is a new producer of the same `Choice`.
- The seam itself: `Selection.FleetIsEndpoint`, and the two
  "gateway routing is not implemented yet" rejections in `routeThroughFleet`
  and `fleet route`.

## Goals / Non-Goals

**Goals:**

- One gateway that is the fleet client wearing a server: selection, waking,
  endpoint resolution, and key handling all come from `internal/fleet`, so the
  routing rules stay written once.
- A request held during a wake behaves like a slow first token, not an error:
  the same readiness wait the launch path uses.
- The launch path's endpoint branch is a small new producer of `fleet.Choice`,
  not a second routing mechanism.
- Every secret keeps its existing home: node tokens and engine keys in the
  environment, named by the fleet file; the gateway's own token supplied the
  way the daemon's is.

**Non-Goals:**

- TLS termination, budgets, spend tracking, per-user keys, retries across
  nodes, request-body logging, queueing. The gateway stays a binary to run,
  not a service to operate.
- New daemon endpoints, and new Spinloop keywords: `servedName` is one
  additive status field, and `FLEET` already takes a URL.
- A gateway config file of its own: fleet.yaml plus flags is the configuration.

## Decisions

### A `gateway` command beside `serve`, a new `internal/gateway` package

`spinloop gateway --fleet ./fleet.yaml --listen :4000` runs in the foreground
like `serve`: it holds a fleet file the way `serve` holds a Spinloop, and its
lifecycle is the process's. Top-level rather than `fleet gateway`, because it
does not observe the fleet the way `fleet status` does; it answers requests
through it, which is a different job.

The HTTP surface lives in `internal/gateway` as an `http.Handler` built from a
`*fleet.Config`, the caller token, and a wake timeout. `cmd/spinloop/gateway.go`
resolves the fleet file, resolves the token, and serves. This keeps the
proxies, the model listing, the wake joining, and the status cache unit-testable
with a fake fleet, the same way the fleet client is tested.

### The gateway wakes a node with the node's own config, never an invented one

A completion request carries one name — the model — and a deploy config needs a
runner, a context, and serve args the request cannot name. The only config a
gateway can push is the one the node was told to run: its Spinloop source,
resolved exactly as `spinloop fleet start` resolves it (the `file` field, a
registered alias named after the node, a same-named directory beside the fleet
file). A node is therefore a wake candidate when it is not running and its
source's config names — as model or served name — the model the request asks
for. This is also the honest semantics: the gateway starts what the fleet file
says each node runs, and says so in the failures it reports.

Consequently `fleet.Wake`'s single-config-per-all-candidates shape does not fit
the gateway, which needs one config per candidate. `Wake` is generalised to
take a per-candidate config function; the launch path passes a constant one
(derived from the Spinloop it wears), the gateway passes a resolver over the
node's own source. The ordering (warm-first), the refusal collection, the
already-running race, and the readiness wait all stay in `Wake`, written once.

### Readiness is a gate before any request is proxied

"Usable now" is three facts: the node reports `running`, the name it reports
serving matches the request, and its engine answers. The third is the daemon's
`Ready` field, and a node reporting not-ready is not a match at all, so
selection skips it — the state turns running when the engine's process exists,
which is before the weights are fetched and loaded, and during that window
nothing is listening on the engine's port. An absent reading is not evidence of
anything: older daemons and runners with no health-check convention report
none, and they still route.

The gateway's per-request shape is therefore: select (from the cached or fresh
fan-out) → when nothing is serving, wait for a node already loading the model,
or wake one if the policy allows → `WaitReady` on the chosen node → proxy. The
wait for a starting node is deliberate: it is loading the weights the request
needs, so it serves sooner than anything a wake would start from cold, and
waking a second node would leave two engines up for one request. It is bounded
by the same wake timeout as a cold start, names the node it holds the request
for, a timeout still leaves the engine running, and it is not a wake — a fleet
whose wake is off still holds a request for a node already loading. The wait
and the wake share one readiness wait: `WaitReady` is lifted out of `Wake`'s
private helper into an exported form that both call.

The same fix closes the client-side gap in the wake race: `Wake`'s
already-running branch currently takes a raced node on the state alone; it now
goes through the readiness wait, so no caller — launch or gateway — is handed
an engine that is still loading. The wait is bounded by the same wake timeout,
and a timeout still leaves the engine running.

### Concurrent cold requests share one wake

With the readiness fix, correctness already holds under concurrent cold
requests: the first starts the engine, the rest lose the 409, re-read, and
wait. But N requests would make N start attempts and N readiness polls against
one wake. The gateway keeps a single in-flight wake per (node, model) — a
mutex-guarded map — and joins it: the first request's wait is the wait the rest
take. At most one start per node, one poll loop, and every request in the
burst is answered from the same engine.

### A short cache over the status fan-out

Selecting fans out over every node, and the round waits on each producer with a
five-second per-node bound: one dead node would add up to five seconds to
*every* request an agent makes. The gateway therefore reuses a fan-out taken
within the last two seconds and re-runs the pure selection per request. The
ranking is on `idleSeconds`, which the fleet-harness-routing design already
calls a crude signal; a two-second-stale reading is unobservable, and the cache
also bounds how often wake decisions are made. The freshness is a package
variable so tests do not wait. A request that loses a race with a stopping node
fails with a connection error naming the node; the next request re-fans-out and
chooses elsewhere, which is the behaviour the no-retry rule gives.

### The proxy: per-request, streaming, key-swapped

Each routed request gets its own single-host reverse proxy at the resolved
engine address, with `FlushInterval` set so a streamed reply is flushed as the
engine sends it rather than in chunks. The proxy's client has no overall
timeout — a long generation is a long response — while the fan-out keeps the
fleet client's short one. The caller's `Authorization` is removed; the engine
key the gateway holds is set as the upstream's when the node reports its engine
gated, and nothing is set when it is not. The body and the reply pass through
unmodified, and a refused or failed upstream reply is the caller's reply: the
gateway does not retry at another node, because the engine's own error is the
honest one.

### Auth and exposure mirror the daemon's rules

One bearer token, the daemon's three sources (file, `SPINLOOP_API_TOKEN`,
command line), more than one a conflict, `401` without the right one, a
non-loopback listen without a token refused at startup, loopback without a
token allowed. The gateway is a longer-lived front door than the daemon's
control API and carries conversation content rather than control traffic, but
it sits on the fleet's own network, whose trust model is already
plain-HTTP-plus-bearer; TLS is an additive listener flag for later, not a
v1 gate. The rule is implemented in the gateway rather than imported from
`internal/daemon`, which keeps the daemon package a leaf the gateway depends
on rather than the other way round.

### Status reports the served name beside the model id

The daemon already stores the deploy config and reports its model id as the
served model; the served name is the same stored config's own field, so status
reports it when the config names one and omits it otherwise — one additive
`omitempty` field, no endpoint change. Older daemons and remotes omit it, and a
gateway matching a request then falls back to the model id alone, which is
exactly today's client-side behaviour. `docs/openapi.yaml` is updated to match;
`openapi_test.go` keeps the two in step.

### The launch path's endpoint branch produces a `Choice`

`routeThroughFleet` gains the branch its rejection used to occupy: a `FLEET`
with a scheme yields a `Choice` whose `BaseURL` is the named endpoint (with the
OpenAI-compatible prefix appended when the value carries no path) and a marker
that it names an endpoint rather than a node. The token is not resolved there:
`applyBeforeLaunch` already builds the resolver chain (environment, `.env`
beside the Spinloop, `ENV` instructions) and already special-cases a fleet
choice's key, so the endpoint's token is resolved through that same chain after
routing, a missing value fails before anything is written naming the variable,
and an already-set variable wins exactly as on the remote path. `fleet route`
gets the same branch as a report: the endpoint has already chosen, no node is
queried, nothing is started.

### `wake:` sits in the fleet file beside `prefer:`

Whether work may be started on the fleet's machines is a property of the fleet,
on the reasoning the `prefer` decision recorded: the same fleet shared by
several people and owned by one person wants different answers, and the fleet
file is the thing that differs. `wake: on|off`, default on (waking is today's
behaviour, so an existing fleet is unchanged), invalid values refused at parse
time. It decides *whether* to wake only: which node is chosen, the ranking, and
what a wake does are all untouched. The launch path's `--no-wake` still wins
over a file that allows waking — an explicit flag beats a file setting — and a
refused wake, by flag or by file, names the node that would have been woken and
the command that would start it.

## Risks / Trade-offs

- **A held request can outlive the caller's patience.** A cold wake is minutes
  and the caller sees a slow response, with no progress to speak to. → It is
  bounded by the wake timeout, the failure says the engine was left running,
  and a retry is answered from the engine that is still loading. The
  alternative — refuse and make the client retry — puts the burden on clients
  that mostly will not retry.
- **A fleet whose nodes name no matching source cannot be woken through the
  gateway**, even if a daemon could run the model from some other config. →
  Deliberate: the gateway starts what the fleet file says a node runs, and the
  failure names the node and the three ways a source could have been given.
  The client-side wake, which carries its own Spinloop, is unaffected.
- **The two-second cache can pick a node that has just stopped.** → The proxy
  fails with a connection error naming the node; the next request re-fans-out
  and chooses elsewhere. No retry, by design.
- **Remotes in the fleet make cold wakes slow** — a scale-from-zero can exceed
  the five-minute default. → Remotes wake through the same uniform `Node`
  interface, and the timeout is a flag: a fleet with remotes sets a longer one
  or declares `wake: off`. A wake that times out on a remote leaves the
  instance starting, as the client-side path already does.
- **`servedName` is absent on older daemons and on remotes**, so aliases do not
  match against them. → Matching falls back to the model id, which is today's
  behaviour; the `no node is serving` failure lists each node's reported model,
  which names the mismatch to anyone who knows the fleet.
- **The gateway holds every node's engine key in its environment.** A machine
  compromise exposes all of them at once. → That is the trade the gateway is
  bought for — the keys stop being distributed to every agent machine — and the
  gateway is the one process the operator runs where the secrets already live.

## Migration Plan

Additive throughout. A fleet file with no `wake:` and a Spinloop with no
endpoint `FLEET` behave exactly as they do now; an older daemon that omits
`servedName` is matched on the model id; the wake race's readiness wait only
ever makes a launch wait longer, never shorter, and only in the race that
previously handed out a loading engine. Rollback is removing the `FLEET` URL
and stopping the gateway process; nothing else changes behaviour unless
something asks it to.

## Open Questions

None that block: TLS, budgets, per-user keys, and request logging beyond a line
per route are excluded by the proposal rather than deferred, and the wake
timeout default (five minutes, shared with the launch path) is a flag, not a
question.
