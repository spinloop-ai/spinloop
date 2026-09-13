## Context

See proposal.md - Why. Relevant existing pieces:

- `cmd/spinloop/fleet.go`'s `runFleetHarness` reads a Spinloop unconditionally
  and fails when none is found and none was given.
- `cmd/spinloop/route.go`'s `routeThroughFleet` already returns a gateway
  `*fleet.Choice` without ever consulting `sel.Model`/`sel.Alias` when the
  fleet file names a gateway — node matching is skipped entirely.
- `cmd/spinloop/main.go`'s `applyRoutedSpinloop` already resolves the
  gateway's token from the environment and wires it into `resolve` before
  calling `applySelection`, which is the one place that currently hard-fails
  with no model or alias.
- `internal/catalog/catalog.go`'s `BuildProviderBlock` already tolerates an
  empty model override (no `models` map, no default model written).
- `internal/catalog/providers.yaml` already has a generic `openai-compatible`
  provider (`apiKeyEnv: OPENAI_API_KEY`), matching the `remoteAPIKeyEnv`
  constant that the gateway's key-resolution plumbing already special-cases.
- opencode's own config format has no live model discovery: its
  `provider.models` map is a static list read once, and opencode has no
  built-in way to query a provider's endpoint at runtime (an open, unresolved
  opencode feature request). A provider block with an empty `models` map
  shows the user nothing to pick. Third-party opencode plugins exist that do
  this via opencode's plugin API, but that is a plugin the user would install
  into opencode itself, not something this Spinloop change can rely on.
- `internal/gateway/gateway.go`'s `handleModels` already implements
  `GET /v1/models` in the OpenAI list shape (`{"object":"list","data":
  [{"id":...,"object":"model"}]}`), authenticated the same way as every other
  gateway route (`Authorization: Bearer <token>`, checked in `authenticate`).
  There is no existing Go client for it — every existing caller is an
  external HTTP client (opencode, curl), not spinloop itself.
- Pi's config format (`internal/pi/pi.go`, `catalog.PiProvider`) has the same
  multi-model shape as opencode: a provider entry holds `Models []PiModel`,
  merged by id, so it can carry a discovered list the same way opencode's
  `provider.models` map does.
- lucinate's cannot: its own doc comments are explicit that "a lucinate
  connection holds exactly one model" (`internal/lucinate/lucinate.go`'s
  `Remove`), and `Connection` has a single `DefaultModel` string, not a list.
  It already tolerates no default model being set at all (`applyConnFields`
  deletes the key when empty) — that is exactly what the model-optional
  waiver alone already gives it.

## Goals / Non-Goals

**Goals:**
- Let `spinloop fleet harness` launch against a gateway with zero local
  Spinloop file.
- Reuse the existing gateway routing, token-resolution, and apply machinery
  rather than adding a parallel code path.
- Keep every other `applySelection` caller (`add`, `apply`, `hf --apply`,
  `spinloop harness`, and fleet-harness-with-a-Spinloop) requiring a model or
  alias exactly as today.
- Populate the harness's model list from the gateway's live `GET /v1/models`
  whenever a gateway-routed selection names no model, for every harness whose
  config format can hold more than one model per provider (opencode, Pi) —
  neither can do this discovery on its own (see Context).
- lucinate still launches successfully against the gateway with no Spinloop
  (via the model-optional waiver), even though it gets no populated list.

**Non-Goals:**
- Changing fleet routing, node selection, or the gateway's `/v1/models`
  listing behavior itself — all already work the way this change relies on;
  only a client for it is added.
- Building or depending on any opencode plugin for live discovery — the model
  list is populated once, at apply time, by spinloop itself.
- Giving lucinate a populated model list. Its config format holds exactly one
  model per connection (see Context); matching opencode/Pi would mean writing
  one managed connection per discovered model, which breaks its documented
  "one connection per provider" design and needs its own id scheme and
  Remove/State changes — considered and explicitly deferred, not pursued
  here. lucinate keeps today's single-model behavior, now optionally empty
  per the model-optional waiver.
- Supporting a modelless launch on the non-fleet `spinloop harness` command.

## Decisions

**Waive the model-or-alias guard with a parameter, not a new function.**
`applySelection` (`cmd/spinloop/main.go`) gains a `gatewayLabel string`
parameter; the guard becomes `if sel.Model == "" && sel.Alias == "" &&
gatewayLabel == ""`. The one caller that can pass a non-empty value is
`applyRoutedSpinloop`, and only when `choice != nil && choice.Gateway` and
the selection names no model or alias — exactly the case routing has already
determined needs no model. The same signal also drives the provider rename
described below, since a string carries both "waive the requirement" and
"here is what to call it" without a second parameter. Alternative
considered: a separate `applyGatewayOnlySelection` function duplicating
catalog-load/apply/print logic — rejected because it would duplicate the
token-resolution and apply steps `applyRoutedSpinloop` already has right.

**Synthesize a bare `spinloop.Selection` rather than writing a temp file.**
When `runFleetHarness` finds no Spinloop and the fleet names a gateway, it
builds `spinloop.Selection{Provider: "openai-compatible"}` directly in Go and
passes it into the existing `applyRoutedSpinloop(sel, "", ...)` path — the
same path a real Spinloop's selection goes through. `path` is `""`, meaning
no directory to look for a `.env` beside (fine: gateway-only launches resolve
their token from the process environment or a `.env` beside the fleet file,
same as today's gateway case). Alternative considered: require the user to
hand-write a minimal Spinloop with just `PROVIDER openai-compatible` —
rejected, since that is exactly the boilerplate this change removes.

**Reuse the `openai-compatible` catalogue provider.** Its `apiKeyEnv` is
`OPENAI_API_KEY`, which equals both `fleet.DefaultGatewayTokenEnv` and the
`remoteAPIKeyEnv` constant that `fleetLaunchResolver` special-cases — so the
gateway's resolved token reaches the harness config's `apiKey` option
correctly regardless of what environment variable name the fleet file's
`gateway.tokenEnv` actually names. No catalogue change needed.

**Carry discovered models through `Selection`, not through the `Harness`
interface.** `Selection` gains a `DiscoveredModels []string` field, documented
the same way the existing `DisplayName` field is: computed at apply time, never
parsed from a Spinloop file. `opencodeHarness.Apply` merges it into the
provider block's `models` map after `BuildProviderBlock` runs; `piHarness.Apply`
merges it into `PiProvider.Models` the same way, after `BuildPiProvider` runs —
in both cases with no default model set, since the point is to leave the
choice to the user. lucinate's adapter ignores the field entirely: its
`Connection` has nowhere to put more than one model (see Context and
Non-Goals). Alternative considered: add a `discoveredModels []string`
parameter to the `Harness.Apply` method itself — rejected, since that forces
lucinate's implementation to take a parameter it cannot use, where `Selection`
already has precedent for a similarly synthetic, apply-time-only field.

**Query the gateway's models once, at apply time, non-fatally.** A new
`fetchGatewayModels(ctx, baseURL, token) ([]string, error)` helper in
`cmd/spinloop` does a plain `GET {baseURL}/models` with the resolved bearer
token and decodes the OpenAI list shape into model IDs. It runs in
`applyRoutedSpinloop`, right after the existing gateway-token resolution,
whenever `choice.Gateway` is true and the selection names no model or alias —
covering both the synthesized no-Spinloop selection and a hand-written
Spinloop with `PROVIDER` but no `MODEL`. A bounded timeout (matching the
existing `remoteEnvTimeout` pattern) keeps a dead gateway from hanging the
launch; on any error, the command warns on stderr and proceeds with an empty
list, the same non-fatal shape `fetchRemoteEnv` already uses for a failed key
refresh — the model list is a convenience for the harness's picker, not
something the gateway needs to route requests. Alternative considered: fail
the launch when the query fails — rejected, since the gateway will still
route per-request correctly even with an empty picker, and refusing to launch
over a listing failure would be worse than an empty list the user can refresh
by relaunching.

**Cosmetic fallout: messages that interpolate the Spinloop path.**
`applyRoutedSpinloop` prints `"Applying %s"` and, on a missing token, `"...set
it in the .env beside %s"`, both using the Spinloop's path. With no Spinloop,
that path is `""`. These fall back to naming the fleet file instead (already
available via `route.fleetFile()`), so the messages stay readable rather than
printing an empty string.

**Found during manual testing: the `.env` lookup itself, not just the
message, must fall back to the fleet file's directory.** The first pass only
fixed the printed text (above) and left `envDir := envFileDir(path)`
unchanged; with `path == ""`, `envFileDir` returns `""`, and
`opencode.EnvResolver("")` defaults to the *current working directory* — not
the fleet file's directory the fixed message now claims. Running the command
from a directory other than the fleet file's (the ordinary case for `--fleet
<path-elsewhere>`) silently missed a `.env` sitting right beside the fleet
file. Fixed by resolving `envDir` to `filepath.Dir(route.fleetFile())`
whenever `path == ""`, so the lookup matches the message. Caught by first
writing a regression test with the fleet file and its `.env` in one temp
directory and the working directory in another — confirmed it failed against
the unpatched code before the fix, then passed after.

**Found from user feedback: the generic provider needs a real name.** Every
gateway-routed selection with no model shared the catalogue's
`openai-compatible` id and its stock "OpenAI-compatible" display name — a
second gateway configured this way would silently overwrite the first's
block, and neither would read distinctly in a model picker next to the
existing "llama.cpp (dev-2)"-style remote-environment entries. Fixed by
mirroring the environment-rename block already in `applySelection`
(`sel.Provider = envName; sel.DisplayName = catalog.RemoteProviderLabel(p.Name,
envName)`): when `gatewayLabel` is set (see above), it renames `sel.Provider`
to `gatewayProviderKey(gatewayLabel)` and sets `sel.DisplayName` the same way
the environment case does. `gatewayLabel` comes from `fleet.Choice.Label`,
itself `GatewayConfig.Label()`: the fleet file's new optional `gateway.name`
field, or the address's host when the section names none — computed in
`internal/fleet` since `Label()` belongs next to the type it describes, not
duplicated in `cmd/spinloop`.

`gatewayProviderKey` prefixes `"gateway-"` onto a slugified label
(`slugify`: lowercase, non-alphanumerics collapsed to one hyphen, trimmed),
falling back to the bare `"gateway"` when the label slugs to nothing (an
edge case with no natural test fixture, kept simple rather than refused).
This key is what opencode's and Pi's config write their provider block under,
and what lucinate's connection id is derived from
(`spinloop:<key>`) — all three already key by `sel.Provider`, so the rename
alone reaches every harness without touching the adapters again.

**Found from user testing: the display name itself needs to say "gateway".**
The provider key (`gateway-<slug>`) carries the word, but the *display name*
— the text a harness's model picker actually shows and searches — did not:
it was built as `catalog.RemoteProviderLabel(p.Name, gatewayLabel)`, and
`p.Name` for the `openai-compatible` catalogue entry is always the generic
"OpenAI-compatible", producing e.g. "OpenAI-compatible (localhost:4000)"
with no "gateway" anywhere in it. A user searching opencode's model picker
for "gateway" found nothing — or worse, an unrelated provider that happened
to contain the word (a remote environment coincidentally named "gateway").
Fixed by passing the literal `"Gateway"` instead of `p.Name` to
`RemoteProviderLabel`, so the same call now reads "Gateway
(localhost:4000)". `p.Name` remains correct for the environment-rename case
just above it, where the engine's real name (e.g. "llama.cpp") is the
meaningful part; it was never meaningful here; the gateway is.

## Risks / Trade-offs

- [A hand-rolled Spinloop with `PROVIDER openai-compatible` and no `MODEL`
  already works stand-alone today via `spinloop harness`, because the same
  `applySelection` guard rejects it too] → out of scope: `spinloop harness`
  keeps requiring a model, since without a fleet's gateway routing there is no
  other source that resolves the model per request; only the fleet-routed
  path gets the carve-out.
- [`applySelection` gains a parameter every future caller must remember to
  set correctly] → kept to a single bool with a doc comment explaining
  exactly when it's `true`; every existing call site is updated explicitly to
  `false` rather than left to a default, so a new caller must make an active
  choice.
- [Every `spinloop fleet harness` run against a gateway now makes an extra
  network call before it can launch] → bounded by a short timeout and
  non-fatal on failure, so the worst case is a brief delay and an empty model
  list, never a hung or failed launch.
- [The model list is only as fresh as the last `spinloop fleet harness` run —
  a node that starts serving a new model afterward will not appear in the
  harness's picker until the user reruns the command] → accepted: this
  matches the "no built-in live discovery" limitation in opencode itself
  (see Context), and reapplying is a normal, cheap action already supported.
- [lucinate users get a working gateway connection but no model list to pick
  from, unlike opencode/Pi users] → accepted as a real gap, not hidden: it
  follows directly from lucinate's own one-model-per-connection design, not
  from an oversight in this change, and is called out explicitly rather than
  silently matched to a lesser experience.
- [Renaming the provider away from `openai-compatible` means the model-key
  path in opencode's config (`<key>/<model>`) changes if the same gateway is
  later given a Spinloop naming a real model under the plain catalogue id, or
  vice versa] → accepted: the two cases were never the same provider block to
  begin with (one keyed by the gateway, one by the catalogue engine), and a
  reapply simply writes a new block rather than silently merging into the
  other one.
