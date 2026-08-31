## Why

A node's tile in `spinloop fleet dashboard` shows its state and outcome only as
plain text ("running", "unreachable", "crashed"...), read left to right
alongside everything else on the tile. Scanning a grid of many nodes for the
ones that need attention means reading every tile's text in full; a glance
cannot tell "healthy" from "not" the way colour can. [Issue #124](https://github.com/spinloop-ai/spinloop/issues/124).

More importantly, `state: running` on its own doesn't mean an engine can
serve requests: the daemon's supervisor flips to `running` the moment the
process starts, before llama.cpp has loaded weights or vLLM has bound its
port. A cloud node started from the dashboard shows the same green a fully
warmed-up node would, for however long weight loading takes. There is
currently no persistent signal — anywhere the dashboard's steady-state
polling reaches — that distinguishes the two.

## What Changes

- The daemon gains a genuine readiness signal: while an engine is running,
  it background-checks the engine's own health endpoint and reports the
  result on both `/v1/status` and `/v1/metrics`, mirroring the existing
  `lastActiveAt`/`idleSeconds` precedent of one shared record feeding both
  endpoints identically.
- Each dashboard tile gets a small coloured status glyph next to the node's
  name, shown in every tile shape (settled, action in flight, not yet
  answered, failed outcome), so health reads at a glance without the border
  colour — which stays reserved for the selection cursor.
- Four health tiers, reusing the project's existing green/yellow/red
  convention from the resource bars (`renderBar` in `cmd/spinloop/remote.go`),
  plus a grey for when no status can be determined at all:
  - **Healthy (green)**: the node answered, its engine is not crashed, and —
    when the daemon reports readiness at all — the engine confirmed ready.
  - **Attention (yellow)**: the node has an action (start/stop) in flight, or
    is `running` with the daemon explicitly reporting the engine not yet
    ready (still loading weights).
  - **Unhealthy (red)**: the node's engine is `crashed`, or its last outcome
    was a failure (`unreachable`, `unauthorized`, `config-error`, `failed`,
    `unsupported`).
  - **Unknown (grey, `?`)**: no status can be determined — the node has not
    yet answered any refresh, or it answered without reporting an engine
    state.
- An older daemon, or a runner with no known health-check convention
  (`omlx`, initially), reports no readiness at all; the tile falls back to
  today's behaviour (green whenever `running`) for that node rather than
  showing a false "attention" that never clears.

## Capabilities

### Modified Capabilities
- `daemon-api`: the status and metrics endpoints gain a readiness field,
  populated while an engine is running and known for its runner, absent
  otherwise (including from a daemon too old to report it).
- `fleet-client`: the dashboard's "Dashboard panels show the node's metrics"
  and related requirements gain a health indicator — every panel SHALL show
  a colour-coded status alongside its existing text, categorized into the
  four tiers above, using the daemon's readiness signal when available.

## Impact

- `internal/daemon/`: a background readiness check against the engine's
  health endpoint, alongside the existing activity sampler; `Ready` added to
  `StatusResponse` and mirrored onto `metrics.Stats`
- `docs/openapi.yaml`: `StatusResponse` and `Stats` schemas gain the new
  field (checked against the implementation by `internal/daemon/openapi_test.go`,
  per `daemon-api-contract`)
- `cmd/spinloop/dashboard_render.go`: `dashTileContent`, a new health-tier
  helper reading the readiness field, the glyph rendering
- `cmd/spinloop/fleet_dashboard_test.go`, `internal/daemon/*_test.go`: fixtures
  and new tests for the readiness check and the tile's tiers
- `openspec/specs/daemon-api/spec.md`, `openspec/specs/fleet-client/spec.md`:
  deltas for the two capabilities above
