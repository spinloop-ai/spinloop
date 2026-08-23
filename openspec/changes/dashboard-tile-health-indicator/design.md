## Context

`dashTileContent` (`cmd/outfit/dashboard_render.go`) builds each panel as a
plain-text block with four mutually exclusive shapes (action in flight, no
refresh yet, failed outcome, settled answer). The only colour on a tile
today is its border, via `lipgloss.Color`, and it encodes selection (214 lit,
240 dim), not health.

Resource bars inside a tile (`renderBar` in `cmd/outfit/remote.go`) already
carry colour, but through raw ANSI escapes concatenated into the plain-text
block, not `lipgloss.Color` — the whole tile body is one string handed to a
single `lipgloss.NewStyle().Render()` call at the border, so per-character
colour inside it has to be ANSI, matching what `renderBar` already does.
`dashClip` (ANSI-aware `ansi.CutWc`) and `lipgloss.Width` already treat these
escapes correctly when clipping/padding tile lines, since the bars go
through the same path today.

The bigger constraint is upstream of the tile: the daemon's `State`
(`internal/daemon/supervisor.go`) flips to `running` the instant the engine
process starts (`supervisor.go:99-108`), before llama.cpp has loaded weights
or vLLM has bound its port. There is no persistent "can this engine actually
serve a request" signal today — only two transient, start-scoped checks:
`fleet.waitReady`/`engineAnswers` (`internal/fleet/wake.go:113-176`, a bare
TCP dial, used only while a launch is in flight) and the cloud Lambda's
`checkHealth` (`remote/lambda/start/index.ts:366-385`, a real `GET /health`
that treats `200`/`401` as ready, also only during `start`). Neither reaches
the dashboard's steady-state polling, which is a plain `GET /v1/metrics`
per node per tick (`dashboard_model.go`'s `refreshRemoteGroup`, hardcoded to
`fleet.MetricsCall` — `fleet.StatusCall` exists but the dashboard never
calls it). See proposal.md - Why.

The daemon already runs one relevant background loop:
`Daemon.SampleActivity` (`internal/daemon/activity.go:88-111`) polls the
engine's token counters on an interval, short until first success
(`catchUpInterval`) then `DefaultSampleInterval` (15s), storing the reading
in a small guarded struct (`engineSample`) that `/v1/metrics` reads instead
of scraping inline — because a busy engine's own `/metrics` blocks behind
its request queue. `d.scrape` (a `metrics.ScrapeTarget{BaseURL, Engine,
APIKey}`) already carries the engine's base URL and runner name at the
moment it's set, alongside each start (`serve_daemon.go`, `SetScrape`).

## Goals / Non-Goals

**Goals:**
- A viewer can tell "this node is up but its engine isn't ready yet" from
  "this node is genuinely healthy," in every tile shape, without reading
  text.
- The readiness signal is real (the engine answered a health check), not
  inferred from timing or heuristics.
- An older daemon, or a runner with no known health-check convention,
  degrades to today's behaviour rather than showing a permanently-yellow or
  falsely-red tile.

**Non-Goals:**
- No legend or footer key explaining the colours — the tiers mirror the
  outcome/state text already on the tile, which remains the source of truth.
- No change to what facts a tile shows, its geometry, or the selection
  border.
- No colour-blindness-specific encoding (e.g. shape variation per tier) —
  out of scope; the existing resource bars have the same limitation.
- No per-runner health-check convention for `omlx` in this change — it has
  no established `/health` path anywhere in the repo (Lambda, catalog,
  engine table); it is explicitly left unchecked (see Decisions) rather than
  guessed at.
- No change to the transient start-time checks (`waitReady`/`engineAnswers`,
  the Lambda's `checkHealth`) — this adds a *steady-state* signal alongside
  them, not a replacement.

## Decisions

### Daemon-side readiness

- **A background health check reusing the existing sample loop**, not a new
  goroutine/ticker. `SampleActivity`'s per-tick loop already does the
  "copy `d.scrape` under the lock, release, then make one HTTP call" dance
  for token counters; a readiness check is called from the same tick,
  alongside `d.sampleOnce(ctx)`. This gets the catch-up cadence (fast until
  a reading lands) for free, and avoids a second background loop with its
  own lifecycle to reason about.
- **Gated by a runner allowlist**, not attempted for every runner. `llamacpp`
  and `vllm` both answer `GET /health` with `200`/`401`-when-ready,
  `503`-or-refused-while-loading (confirmed in the Lambda's `checkHealth`
  and its comment, which already applies this identically to both). `omlx`
  has no established health path anywhere in this repo. Attempting the
  check against an unknown convention would report every `omlx` node
  "not ready" forever — worse than not checking at all — so the check is
  skipped for any runner not in the allowlist, and the field stays absent
  for it, same as an older daemon.
  - Alternative considered: attempt the check universally and treat any
    non-2xx/401 as "not ready." Rejected — this would misreport `omlx`
    (and any future runner) as permanently unready, actively worse than the
    status quo of no signal at all.
- **Unauthenticated probe, `200` or `401` both count as ready.** A key-gated
  engine correctly rejects an unauthenticated request with `401`; treating
  that as "not ready" would report every gated engine as stuck. This
  mirrors the Lambda's own rule exactly, so cloud and local readiness agree.
- **One shared record feeds both `/v1/status` and `/v1/metrics`**, the same
  pattern `lastActiveAt`/`idleSeconds` already uses (`daemon-api`'s
  "Metrics reports engine activity" requirement) — a small guarded struct
  (`readiness`, mirroring `engineSample`'s shape: `record`/`read`/`forget`),
  read by both handlers, `forget()`-ed on each new `StartEngine` alongside
  `d.sample.forget()` so a previous engine's answer is never attributed to
  the one that replaced it.
- **A tri-state string field (`"ready"` / `"not-ready"` / absent), not a
  `bool`.** The chosen sketch used `Ready bool`, but a `bool` with
  `omitempty` cannot distinguish "confirmed not ready" from "field absent" —
  both are the zero value and both get omitted from the JSON. That
  collapses exactly the two cases this change needs to tell apart: a daemon
  that has checked and found the engine still loading, versus an older
  daemon (or an unchecked runner) that has no opinion at all. A string with
  `omitempty` keeps `""` (absent, unknown) distinct from an explicit
  `"not-ready"`, while still degrading cleanly: an old daemon's JSON simply
  lacks the key, which unmarshals to `""`, the same value used for "not
  applicable." No pointer type is needed, and no version-comparison logic is
  introduced — none exists elsewhere in this codebase, and this field
  doesn't need to be the one to start that pattern.
- **`docs/openapi.yaml` gains the field on both `StatusResponse` and `Stats`
  schemas**, required by `daemon-api-contract`'s existing "description is
  verified against the implementation" requirement (`internal/daemon/openapi_test.go`
  compares Go struct fields against the YAML schema's properties) — this is
  enforcement already in place, not new process.

### Dashboard tile

- **A coloured glyph beside the name, not the border.** The border already
  means "selected"; overloading it with health would make selection
  ambiguous with health at the moment they coincide. A separate glyph keeps
  the two orthogonal, per the chosen UX option.
- **Raw ANSI escapes for the glyph colour**, matching `renderBar`, not
  `lipgloss.Color`. The tile body is built as a plain string via
  `fmt.Fprintf`/`strings.Builder` and only wrapped in a lipgloss style once,
  at the border — `lipgloss.Color` needs its own `.Render()` call per
  segment, which `dashTileContent` doesn't do elsewhere in the body.
- **Three tiers, computed once per tile from `(fleet.NodeResult, dashAction)`**,
  in priority order: an action in flight is always "attention" regardless of
  the last refresh; then no refresh yet is "attention"; then a failed
  outcome or a `crashed` engine is "unhealthy"; then `running` with the
  daemon explicitly reporting `Ready == "not-ready"` is "attention"; else
  "healthy" — which covers both a confirmed-ready `running` node and a
  `running` node whose daemon reports no readiness at all (old daemon,
  unchecked runner), matching this change's degrade-gracefully goal.
  - Alternative considered: a boolean healthy/unhealthy instead of three
    tiers. Rejected — the dashboard's existing text already distinguishes
    "no data yet" from "actively failing," and collapsing that into a
    single "not healthy" bucket would report a fleet reachable-but-not-yet-
    tested as red at cold start, alarming when nothing is actually wrong.
  - Alternative considered: colour the state/outcome word itself instead of
    a new glyph. Rejected in the UX decision (see proposal) because it
    conflates "this is failure text" with "this is the label text," and the
    settled tile's state word is followed by uptime on the same line with
    no natural colour break.
- **Glyph is prepended to the tile's very first line** in every shape (the
  name line), immediately before the name, with a following space — the one
  line every shape has in common, so the tier is visible even when a tile
  shows nothing else yet (`waiting for first refresh…`).

## Risks / Trade-offs

- [Byte-stable tests assert exact tile text and will need the glyph and its
  ANSI colour added to every fixture] → Update
  `cmd/outfit/fleet_dashboard_test.go`'s expected strings as part of this
  change; `dashTileExpected`/`lipgloss.Width` already handle ANSI-aware
  padding, so no test-helper changes are needed.
- [Adding two columns ("● ") to the name line shrinks the room for a long
  node name before `dashClip` cuts it] → Acceptable: node names are
  operator-chosen and short in practice, and the line was already shared
  with the state/outcome text.
- [The readiness check adds one more HTTP call per sample tick, per node,
  against the engine itself] → Bounded the same way the token scrape
  already is: only while `running`, only on the existing interval, with the
  same 5s client timeout (`scrapeClient` in `internal/metrics/scrape.go`) —
  no new load pattern, just one more cheap request alongside one already
  made.
- [A `503`-while-loading engine and a genuinely wedged engine both read
  "not-ready" forever, with no distinction] → Accepted as consistent with
  today's `State: running` also not distinguishing those two cases; a wedged
  engine already shows as "running" with no counters moving, which the
  operator diagnoses the same way regardless of this change.
- [`omlx` nodes get no readiness signal, so a viewer might expect the same
  precision they get for llamacpp/vllm] → Documented as a known gap in the
  proposal; the tile falls back to today's state-only behaviour for those
  nodes rather than a misleading colour.
