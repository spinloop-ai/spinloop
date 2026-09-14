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
failing with a generic "gave up waiting" message. The "undeployed refuses
promptly" half of this change does need a check on the Go side because of
that: `StartWith` reads a fresh status first and refuses immediately when it
reports nothing served, rather than handing an undeployed environment to the
retry loop.

## Goals / Non-Goals

**Goals:**
- A deployed-but-stopped remote node becomes a wake candidate for the
  gateway's `/v1/models`, its topology endpoint, and its request-time wake,
  sourced from the node's own last status rather than a local file.
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

Resolve it from the node's own last-read status (`NodeResult.Status`, already
gathered by the fan-out every caller already runs), not from a local Spinloop
source. `statusFromRemote` already carries `Model` and `ServedName` from the
environment's stored deploy config on every status read, deployed or not,
running or not — that is the authoritative record of what the environment
would serve, and it is already in hand wherever a candidate list is built.

Rejected alternative: give a remote node entry a local Spinloop source and
resolve its config the way a daemon node's is. Rejected because a remote
environment is deployed with `spinloop remote deploy`, a separate flow that
can drift from whatever local file the fleet entry happens to point at (or
point at nothing); trusting the control plane's own record avoids a second
copy of "what does this node run" that could disagree with the first.

This resolution is added as a small remote-aware branch on top of the
existing `ConfigFor` the gateway already injects into `fleet.Wake` and
`wakeableModels` — built once per request from the `results` already in
scope, since a `ConfigFor` is `func(NodeConfig) (inference.DeployConfig,
error)` and has no other way to see live status. `internal/fleet` itself
(`wake.go`, `configResolver`, `candidates`, `WouldWake`) is untouched: it
already only knows about "the injected resolver said X" and does not care
whether X came from a file or from a status read. The existing "does the
resolved config match what the request wants" check
(`gateway.matchingConfigFor`) keeps wrapping that combined resolver
unchanged, so it enforces the match for a remote candidate exactly the way
it already does for a daemon one.

### Starting a deployed remote node

`remoteNode.StartWith` stops refusing and delegates to the same
`StartWithProgress` that `Start` already uses, ignoring the `dc` and
`engineKey` it is handed: a remote environment's engine is gated by the key
fixed at deploy time, and what it serves is fixed by its own stored deploy
config, not by anything a wake call supplies. Before that call, `StartWith`
reads a fresh status and refuses immediately, naming `spinloop remote
deploy`, when it reports nothing served (see Context on why this cannot be
left to the boot call's own `503`). This is the same "nothing deployed"
signal `remoteConfigFor` already reads from a status reply elsewhere in this
change, so the two layers agree on what "undeployed" means without sharing
code — one reads it from the gateway's already-fanned-out `results`, the
other from a fresh call because `StartWith` is not handed a `NodeResult`.

Rejected alternative: check `dc` against the environment's last known model
before calling `StartWithProgress`, refusing locally on a mismatch. Rejected
as unnecessary — a remote candidate only reaches `StartWith` after the
gateway's own match check already accepted its last-known config as
matching the request, and duplicating that check here is validating a
condition the caller already enforced.

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

## Risks / Trade-offs

- [A remote candidate's last-known status is stale by up to the reading's
  cache window, so a wake could be attempted against a model the
  environment was just re-deployed away from] → Mitigation: the same
  staleness already applies to a daemon node's running state, and the
  control plane's own reply is still the last word — a mismatch surfaces as
  the started engine not matching what was asked, the same class of race
  routing already tolerates elsewhere.
- [A second place (`node.wake`) now decides whether a node wakes, which
  could confuse debugging] → Mitigation: refusal and topology text names
  which setting decided it.
- [Remote's `isAlreadyRunning` race handling in `wake.go` was written and
  tested against a daemon's `409` and error text; a remote start's conflict
  may not present the same way] → Mitigation: out of scope for this change
  — worst case a race is reported as a refusal rather than adopted gracefully,
  which is a missed optimisation, not incorrect behaviour, and matches how
  a remote node's races are already handled everywhere else in the fleet
  package today.
