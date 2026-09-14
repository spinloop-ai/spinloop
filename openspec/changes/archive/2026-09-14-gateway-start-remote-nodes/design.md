## Context

See proposal.md for the motivation. Three things in the current code make the
refusal happen, and the fix touches each differently:

- `remoteNode.StartWith` (`internal/fleet/remote_node.go`) refuses
  unconditionally, before anything is checked against the control plane.
- `gateway.wakeableModels()` (`internal/gateway/gateway.go`) only reads a
  node's wakeable model by resolving its local Spinloop source (`h.cfgFor`,
  wired up in `cmd/spinloop/gateway.go` via `resolveNodeSpinloop` +
  `readSpinloop`), and explicitly skips any node whose `Kind != KindDaemon`.
- `fleet.Config.Wake` (`internal/fleet/wake.go`) resolves each candidate's
  deploy config the same way, through the injected `ConfigFor`, so a remote
  candidate either has no resolvable local source or, if it does, is starting
  from a config that is not what the environment already has deployed.

Separately: `spinloop fleet start <remote-node>` already boots a deployed
remote environment today, through `Node.Start` (not `StartWith`) in
`cmd/spinloop/fleet.go`'s `fleetStartCall`. That path is unaffected by this
change and is the proof that booting a deployed environment with no pushed
config already works end to end.

The remote start Lambda (`remote/lambda/start/index.ts`) already reads the
environment's own stored deploy config on every wake and already answers,
with a `503` naming `spinloop remote deploy`, when nothing is deployed
(`state: 'unconfigured'`) or nothing is provisioned (`state: 'undeployed'`).
That reply is not an immediate failure on the Go side, though:
`remote.Start` (`internal/remote/remote.go`) treats every `503` alike —
still booting, no capacity, or undeployed — and retries after the reply's
`retry_after_seconds` until its own deadline, so an undeployed environment
would otherwise hold a wake request for the whole wake timeout before
failing with a generic "gave up waiting" message. That means a candidate
must be confirmed deployed *before* `StartWith` is ever called, not by
`StartWith` itself — see the next point for why it also cannot be confirmed
from a status read.

The status Lambda's own non-running branch (`status()`, same file) answers a
stopped or undeployed environment with only `{state, environment, healthy,
base_url}` — no runner, model id, or served name; those only ride along
(`readDeployFacts`, spread into the reply) once the instance is `running`.
So a fan-out's cached status for a stopped environment cannot say what it
would serve, deployed or not — the two look identical. The stats Lambda
(`remote/lambda/stats/index.ts`) is different: it reads the deploy config
directly (`readDeployConfig`) before it even looks at instance state, and
fails outright (no instance-state branch reached at all) when there is none
to read. Its reply carries `runner` and `modelId` in every branch it does
reach, running or not — but never a served name, which only the deploy
config's own field name (not relayed by this Lambda) would supply.

## Goals / Non-Goals

**Goals:**
- A deployed-but-stopped remote node becomes a wake candidate for the
  gateway's `/v1/models`, its topology endpoint, and its request-time wake,
  sourced from the node's own stats reply rather than a local file.
- An operator can allow or refuse waking on one node without changing the
  fleet's own `wake` setting.

**Non-Goals:**
- Undeployed remote environments stay out of scope, per the issue: the
  refusal for them is preserved, just moved to where the control plane
  already produces it rather than duplicated in Go.
- `spinloop fleet start`, `fleet route`, and the orchestrator's admission
  (#188) are unaffected — they already treat a remote node correctly for
  their own purposes and are not part of this change.
- No change to how a daemon node is woken or how its config is resolved.

## Decisions

### Where a remote node's wakeable config comes from

Resolve it from a live call to the environment's stats endpoint
(`Node.Metrics`, wrapping `remote.Stats`), not from the status a fan-out
already holds and not from a local Spinloop source. The status reply is no
good for this (see Context: it carries deploy facts only while running), so
this is a genuine extra network call rather than a free read of data already
in hand — bounded the same way a daemon node's Spinloop-file read is, by the
gateway's existing `sourcesTTL` cache on `wakeableModels`, and by `Wake`'s
own per-node-name memoisation (`configResolver`) within one wake attempt.

Rejected alternative: give a remote node entry a local Spinloop source and
resolve its config the way a daemon node's is. Rejected because a remote
environment is deployed with `spinloop remote deploy`, a separate flow that
can drift from whatever local file the fleet entry happens to point at (or
point at nothing); trusting the control plane's own record avoids a second
copy of "what does this node run" that could disagree with the first.

Because this needs a live call — building the node from the registry and
reaching its control plane — it is a method on `Handler`
(`remoteConfigFor(ctx) fleet.ConfigFor`), not a pure function over `results`:
it needs `h.cfg.NewNode` and a `context.Context` to bound the call, neither
of which a `results []fleet.NodeResult` slice carries. `wakeableModels` and
`wakeFor` pass a request-scoped context through; the config-resolution
signature otherwise stays the same shape (`fleet.ConfigFor`), so
`internal/fleet` itself (`wake.go`, `configResolver`, `candidates`,
`WouldWake`) is still untouched — it only knows "the injected resolver said
X" and does not care whether X came from a file or a live stats call. The
existing "does the resolved config match what the request wants" check
(`gateway.matchingConfigFor`) keeps wrapping the combined resolver
unchanged, so it enforces the match for a remote candidate exactly the way
it already does for a daemon one — on `ModelID` alone for a remote node,
since the stats reply carries no served name to match on.

### Starting a deployed remote node

`remoteNode.StartWith` stops refusing and delegates to the same
`StartWithProgress` that `Start` already uses, ignoring the `dc` and
`engineKey` it is handed: a remote environment's engine is gated by the key
fixed at deploy time, and what it serves is fixed by its own stored deploy
config, not by anything a wake call supplies. It does not itself check
whether the environment is deployed — a status read cannot tell that apart
from stopped-and-deployed (see Context), so a check here would inherit the
same blind spot the original design mistakenly built into it. That
confirmation already happened one step earlier, in the gateway's own
candidate matching (`remoteConfigFor`, this document's other decision),
which reads the stats reply and only offers a candidate whose deploy
config actually resolved — `Wake`'s loop never reaches `StartWith` for a
node that check refused. `internal/fleet.Wake` has exactly one caller of
`StartWith` on a remote node, so there is nowhere else that gap could be
reached from today.

Rejected alternative: check `dc` against the environment's last known model
before calling `StartWithProgress`, refusing locally on a mismatch. Rejected
as unnecessary — a remote candidate only reaches `StartWith` after the
gateway's own match check already accepted its resolved config as matching
the request, and duplicating that check here is validating a condition the
caller already enforced.

### Per-node wake override

Add `NodeConfig.WakePolicy` (yaml `wake`, same `on`/`off` shape and parse
validation as the fleet-wide setting) and `Config.NodeWakes(entry) bool`:
the node's own setting when it names one, else the fleet's — plus
`Config.AnyNodeWakes() bool` for a caller that needs to know whether waking
is possible at all before it tries.

The check itself lives inside `Wake`'s own per-candidate loop, not in
`wakeable()`'s ordering: a candidate whose own wake is off is skipped there
the same way a candidate whose config doesn't match is skipped, contributing
"waking is disabled for this node" to the refusal `Wake` reports when
nothing else can serve the request either. `wakeable()` and `WouldWake`
stay exactly as they were — reporting the node that would be tried first on
config alone, regardless of policy. That is deliberate: `WouldWake` is what
`fleet route`'s and the gateway's pre-emptive refusal already used to name
the node a wake-off setting was refusing, and a caller doing that still
needs the answer to "who would this have woken" even when the answer is
"nobody, because policy said no" — folding the policy into `wakeable()`
itself would have made `WouldWake` blind to that node whenever the fleet or
the node's own setting is off, breaking the exact message it exists to
produce.

The gateway's request-time refusal (`wakeFor`) and its advertisement
(`wakeableModels`) switch from the single `h.cfg.Wakes()` check to,
respectively, `h.cfg.AnyNodeWakes()` (is there any point trying `Wake` at
all) and `h.cfg.NodeWakes(entry)` per node (does this one contribute to what
a request could start). `cmd/spinloop/route.go`'s own pre-emptive refusal —
outside this change's stated scope, but sharing the same fleet-wide-only gate
this change touches — gets the same `AnyNodeWakes()` swap, so a per-node
override is not silently inert there.

Rejected alternative (from the issue): a second fleet-wide flag scoped to
remote nodes only (e.g. `wakeRemote`). Rejected per the chosen direction — a
per-node override is more general (it also lets a daemon node opt out
individually) and reads directly off the node it affects rather than adding
a second global switch whose interaction with the first has to be
documented separately.

### Coalescing concurrent wakes

Found running this against a real fleet: two requests landed on the gateway
within half a second of each other, both wanting the same model nothing was
serving, and both logged "Waking dev-4...". A daemon node's control API
would have turned the loser into a joiner via its own `409` — that race was
never actually a problem for a daemon node, so the original design (see the
now-corrected risk note above) assumed a remote environment's conflict would
present the same way and left it alone. It does not: `remote/lambda/start`'s
only de-duplication is a `DescribeInstances` tag lookup, which is eventually
consistent right after a `RunInstances` call, so two `wake()` invocations
close enough together can each miss the other's not-yet-visible instance and
each launch one — a real double-billed launch, not a cosmetic double log
line, and never surfaced as an error either side could catch: the Lambda
doesn't propagate a failed on-instance daemon start as anything a caller
would recognise as a conflict.

Fixed with a `golang.org/x/sync/singleflight.Group`, keyed by the fleet
file's path alongside the node's name, wrapping exactly the "start (or join)
and wait for ready" step inside `Wake`'s per-candidate loop — the same step
the daemon's `409` used to make redundant for daemon nodes and now makes
redundant for every node kind uniformly. Two concurrent calls for the same
node share one call to `startAndWait`; only the first actually starts
anything, and both receive its result. The shared call runs on its own
`context.Background()`, not either caller's request context: whichever
caller happened to be first must not have the wait cut short by its own
request being cancelled while another caller is still waiting on the same
node, and `waitReady` already bounds the wait by `WakeTimeout` on its own
regardless of context.

A fatal outcome (the engine started, or was found already running, but
never answered) has to reach every caller sharing that result the same way
a non-fatal refusal does not: the former must not be retried against another
candidate — the engine is left running — while the latter should let each
caller's own loop move on to its own next candidate. `startAndWait` signals
this with a `*fatalWakeError` wrapper so `Wake`'s loop, after `Do` returns,
can tell the two apart without singleflight itself needing to know anything
about wake-specific semantics.

Rejected alternative: fix the race in the remote Lambda instead (a
DynamoDB-backed idempotency lock, or Lambda reserved concurrency of 1 per
environment). Rejected for this change — it would need a coordinated
`remote/` CDK redeploy per environment, which this PR cannot cause on its
own, whereas the Go-side fix closes the gap for every already-deployed
environment the moment the gateway binary updates. A control-plane-side lock
would still be worth doing eventually, as defence in depth against a
caller outside this gateway process (a second gateway instance, a direct
script) racing the same environment — out of scope here.

### Trusting a real readiness signal over an open port

Found once dev-4 actually booted: the gateway routed a request to it while
the model was still loading, and the request failed. `waitReady` was built
to prefer a real readiness reading over a raw TCP probe — the probe only
checks that a port accepts a connection, which both llama.cpp and vLLM do
before they can answer a request — but its `if status.Ready ==
daemon.ReadyYes { return }` fell through to the probe for *everything else*,
including an explicit `ReadyNo`, not only "no reading landed". A daemon node
rarely surfaces this: its own reading is almost always populated one way or
the other by the time a caller asks. A remote node's `Ready` was *always*
empty going into this fix, for an unrelated reason — `statusFromRemote`
never mapped the control plane's own `healthy` field (the same `/health`
check, already relayed on a running remote view) onto it at all — so every
remote wake fell through to the probe by construction, making the gap
certain rather than occasional. Fixing the mapping without also fixing the
fallthrough would have made the gap visible without closing it: a properly
populated `ReadyNo` would still have lost to a successful probe.

Both are fixed together: `statusFromRemote` now maps `Healthy` onto `Ready`
the way a local daemon's own check is reported, and `waitReady` branches on
`Ready` three ways — `ReadyYes` returns, `ReadyNo` waits without probing,
and only an empty reading falls back to the probe, matching what the
existing routing check for an already-running node (`select.go`'s
`running()`, which already treated `ReadyNo` correctly) established was the
right rule. No design alternative was considered here — this was a
correctness bug against the design's own stated intent, not a choice.

## Risks / Trade-offs

- [A remote candidate's resolved config is stale by up to `wakeableModels`'
  cache window on the models/topology paths (30s) — not on the wake path
  itself, which resolves fresh — so a listed model could lag a re-deploy
  briefly] → Mitigation: the same staleness already applies to a daemon
  node's Spinloop-file read under the same cache, and the control plane's
  own reply when the wake is actually attempted is still the last word.
- [Every stopped remote node now costs a live stats call each time
  `wakeableModels` re-resolves (every `sourcesTTL`), instead of a free read
  of already-fanned-out status] → Mitigation: bounded by the same cache a
  daemon node's file read already relies on; a remote environment missing a
  configured `stats_url` fails that one node's resolution the same way a
  daemon node with no resolvable Spinloop source does, rather than the
  whole reply.
- [A second place (`node.wake`) now decides whether a node wakes, which
  could confuse debugging] → Mitigation: refusal and topology text names
  which setting decided it.
- [Two requests racing to wake the same remote node could each miss the
  other's not-yet-visible instance in the control plane's eventually
  consistent instance lookup, launching two — this was flagged here as
  "out of scope, worst case a missed optimisation" on the assumption that a
  remote start's conflict would surface as a refusal the way a daemon's 409
  does; it does not (see "Coalescing concurrent wakes" below), so this was
  wrong and had to be fixed rather than accepted] → Mitigation: see
  "Coalescing concurrent wakes".
