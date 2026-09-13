## Why

`spinloop fleet harness` requires a Spinloop even when the fleet file names a
gateway, and fails with "a launch needs a Spinloop to know which model to
route" if none is found. That requirement exists for the node-routing path,
where the router matches a node by the model the Spinloop names. But routing
at a gateway already skips node matching entirely: `routeThroughFleet` returns
a gateway choice without ever reading the Spinloop's model, because the
gateway resolves the model per request and its `/v1/models` endpoint already
lists every model the fleet's nodes serve. Requiring a Spinloop only to name a
model the gateway ignores is a leftover restriction from the node-routing
case, and it means every gateway-only launch needs a throwaway Spinloop file
with no real model in it.

Simply applying no model at all is not enough on its own: opencode's config
format has no notion of a provider with an undetermined model list — its
`models` map is a static list read once at startup, and opencode has no
built-in way to query a provider's endpoint for what is available (an open
opencode feature request, unresolved as of this writing). Left with an empty
`models` map, opencode would show the gateway's provider with nothing to pick.
So the harness's model list needs to be populated some other way, and the
gateway's own `/v1/models` — already built to answer exactly this — is the
natural source: `spinloop fleet harness` can query it once at apply time and
write the result into opencode's config, refreshed on every launch.

## What Changes

- `spinloop fleet harness`, when the fleet file names a gateway and no
  Spinloop is given (none named via `-O`/a leading argument, and none found
  beside the fleet file), no longer fails. It configures the active harness
  with a generic OpenAI-compatible provider pointed at the gateway's base URL,
  resolves the gateway's token via the environment the same way the existing
  gateway path does, and queries the gateway's `GET /v1/models` to populate
  the harness's model list — still with no single default model, since the
  point is to leave the choice to the user.
- The same live query runs for any gateway-routed selection that names no
  model or alias (for example a hand-written Spinloop with just `PROVIDER
  openai-compatible`), not only the no-Spinloop case, since both reach the
  same apply step with nothing to route by model.
- A failure to reach the gateway's `/v1/models` (down, timed out, bad
  response) is not fatal: the harness is still applied, with a warning and an
  empty model list, the way a failed remote-key fetch already warns and
  carries on elsewhere in this codebase.
- When a Spinloop names a model or alias, or the fleet names no gateway (so a
  node must be matched by model), behavior is unchanged: the existing "needs a
  Spinloop" failure still applies for node-based routing, and a named
  Spinloop's model is still applied as today with no live query.
- No change to `spinloop harness` (the non-fleet launch), to fleet routing
  itself, or to the gateway's own model-listing behavior. opencode's and Pi's
  adapters both gain the populated-list behavior, since both config formats
  hold more than one model per provider. lucinate's does not — it holds
  exactly one model per connection — so it keeps today's behavior beyond the
  model-optional waiver already planned: a working gateway connection with no
  pre-populated model.
- A gateway-routed selection with no model or alias is written under a
  provider key and display name derived from the gateway, not the catalogue's
  shared `openai-compatible` id — otherwise every gateway a fleet might name
  would collide under one block, and none would read distinctly in a model
  picker. The fleet file's `gateway` section gains an optional `name` field to
  label it explicitly (e.g. "OpenAI-compatible (remote-llms)"); with none
  given, the gateway's address stands in (e.g. "OpenAI-compatible
  (localhost:4000)") — the same pattern a remote environment already reads as
  ("llama.cpp (dev-2)").

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-client`: "The fleet harness command" requirement's "no Spinloop, no
  route" rule gains a gateway carve-out — a launch through a gateway with no
  Spinloop no longer fails, and configures the harness by querying the
  gateway's live model list instead of naming one, under a provider key and
  display name derived from the gateway rather than the shared catalogue id.
- `fleet-config`: "A fleet file MAY name a gateway" gains an optional `name`
  field in the `gateway` section, labelling it for the harness-naming
  behavior above.

## Impact

- `cmd/spinloop/fleet.go`: `runFleetHarness` builds a bare provider selection
  instead of failing when no Spinloop is given and the fleet names a gateway.
- `cmd/spinloop/main.go`: `applySelection`'s model-or-alias requirement
  becomes conditional on a new `gatewayLabel string` parameter, non-empty
  only for a selection routed at a gateway with no model or alias of its
  own; three unrelated call sites (`spinloop add`, `spinloop apply`,
  `spinloop hf --apply`) are updated to keep requiring a model or alias. A
  new client call queries the gateway's `GET /v1/models` when a
  gateway-routed selection names no model, on the same non-fatal-warning
  terms as the existing remote-key fetch.
- `internal/spinloop/spinloop.go`: `Selection` gains a `DiscoveredModels`
  field, computed at apply time like the existing `DisplayName` field — never
  parsed from a Spinloop file.
- `internal/harness/adapters.go`: `opencodeHarness.Apply` merges
  `DiscoveredModels` into the provider block's `models` map when present, and
  `piHarness.Apply` merges it into `PiProvider.Models` the same way. The
  lucinate adapter is untouched — its config format holds one model per
  connection, with no list to populate.
- `internal/catalog/providers.yaml`'s existing `openai-compatible` entry is
  reused as the provider for the synthesized selection; no catalogue change.
- Test coverage in `cmd/spinloop` for the new no-Spinloop/gateway path
  (including a mock gateway for the models query, and its failure mode), and
  for the unchanged no-gateway/no-Spinloop failure.
- A follow-up fix, found in manual testing: with no Spinloop, the `.env`
  lookup for the gateway's token must resolve beside the fleet file, not the
  working directory the command happens to run from — the "no token" error
  already claimed the former, but the code fell back to the latter.
- `internal/fleet/config.go`: `GatewayConfig` gains a `Name` field and a
  `Label()` method (explicit name, else the address's host);
  `internal/fleet/select.go`'s `Choice` gains a `Label` field, set from
  `gw.Label()` in `cmd/spinloop/route.go`'s `routeThroughFleet` and carried
  through to the apply step.
- `cmd/spinloop/main.go`: a new `gatewayProviderKey`/`slugify` pair turns a
  gateway's label into a provider key (`"gateway-" + slug`, or bare
  `"gateway"` if the label slugs to nothing); `applySelection` renames the
  provider and sets its display name under the same `gatewayLabel` signal
  that waives the model-or-alias requirement, mirroring the existing
  `envName` rename block.
