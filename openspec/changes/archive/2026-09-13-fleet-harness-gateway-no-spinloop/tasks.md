## 1. Waive the model-or-alias guard for gateway-routed selections

- [x] 1.1 Add a `modelOptional bool` parameter to `applySelection`
      (`cmd/spinloop/main.go`); change the guard to
      `if sel.Model == "" && sel.Alias == "" && !modelOptional`. Update its
      doc comment to say when `true` is passed. Verify: `go build ./...`
      succeeds.
- [x] 1.2 Update the three unrelated call sites (`cmd/spinloop/hf.go`'s
      `--apply` path, `cmd/spinloop/main.go`'s `addCmd`, and `applyCmd`) to
      pass `false`. Verify: existing tests covering "needs a model or an
      alias" (`cmd/spinloop/flagparse_test.go`) still pass unchanged.
- [x] 1.3 In `applyRoutedSpinloop` (`cmd/spinloop/main.go`), pass
      `choice != nil && choice.Gateway` as `modelOptional` to
      `applySelection`. Verify: `go test ./cmd/spinloop/...` passes.

## 2. Fix messages that assume a Spinloop path

- [x] 2.1 In `applyRoutedSpinloop`, replace the bare `path` interpolated into
      the `"Applying %s"` print and the "no token to reach the gateway ...
      beside %s" error with a label that falls back to `route.fleetFile()`
      when `path == ""`. Verify: manually trace both messages for the
      no-Spinloop gateway case and confirm neither prints an empty string.
- [x] 2.2 Fix `envDir` itself, not just the message: with `path == ""`,
      resolve it to `filepath.Dir(route.fleetFile())` instead of leaving it
      empty (which made `opencode.EnvResolver` look in the working directory,
      contradicting the message from 2.1). Found via manual testing against a
      real fleet file outside the working directory. Verify: a regression
      test with the fleet file and its `.env` in one directory and the
      working directory in another — confirmed to fail before the fix and
      pass after.

## 3. Synthesize a bare selection when no Spinloop is given and a gateway is named

- [x] 3.1 Add a `gatewayProviderID = "openai-compatible"` constant in
      `cmd/spinloop/fleet.go`.
- [x] 3.2 In `runFleetHarness`, when `readSpinloop` fails and no Spinloop was
      given: resolve the fleet config, and if it names a gateway
      (`cfg.GatewaySection()`), build
      `sel = spinloop.Selection{Provider: gatewayProviderID}` with
      `resolvedPath = ""` and continue into the existing
      `applyRoutedSpinloop` call instead of returning the "needs a Spinloop"
      error. If fleet resolution fails, or the fleet names no gateway, keep
      today's error. Verify: `go build ./...` succeeds.
- [x] 3.3 Update `fleetHarnessCmd`'s `Long` help text and the
      `runFleetHarness`/`applyRoutedSpinloop` doc comments to describe the
      no-Spinloop gateway case. Verify: `spinloop fleet harness --help` reads
      correctly.

## 4. Populate the harness's model list from the gateway's live models endpoint

- [x] 4.1 Add `DiscoveredModels []string` to `spinloop.Selection`
      (`internal/spinloop/spinloop.go`), documented like the existing
      `DisplayName` field: computed at apply time, never parsed from a
      Spinloop file. Verify: `go build ./...` succeeds.
- [x] 4.2 Add `fetchGatewayModels(ctx context.Context, baseURL, token string)
      ([]string, error)` in `cmd/spinloop` (near the other gateway-key
      helpers in `main.go`): `GET {baseURL}/models` with `Authorization:
      Bearer <token>`, decode the OpenAI list shape
      (`{"object":"list","data":[{"id":...}]}`) into an ordered slice of IDs,
      under a bounded timeout matching the existing `remoteEnvTimeout`
      pattern. Verify: a unit test against an `httptest.Server` returning a
      models list, and one returning a non-200/malformed body, both decode or
      error as expected.
- [x] 4.3 In `applyRoutedSpinloop`, after the existing gateway-token block,
      when `choice.Gateway` is true and `sel.Model == "" && sel.Alias == ""`:
      call `fetchGatewayModels` with `choice.BaseURL` and the resolved token;
      on success set `sel.DiscoveredModels`, on error print a warning to
      stderr and leave it empty — neither case fails the command. Verify: a
      test with a mock gateway confirms both branches, and that a fetch
      failure still results in a successful launch.
- [x] 4.4 In `opencodeHarness.Apply` (`internal/harness/adapters.go`), after
      `catalog.BuildProviderBlock` runs, merge `sel.DiscoveredModels` into
      `block["models"]` (creating the map if absent, one entry per ID, no
      entry marked as the default). Verify: a test asserts the written
      opencode config's `provider.<id>.models` map contains the discovered
      IDs and the top-level `model` key is absent.
- [x] 4.5 In `piHarness.Apply` (`internal/harness/adapters.go`), after
      `catalog.BuildPiProvider` runs, merge `sel.DiscoveredModels` into
      `prov.Models` the same way (one `PiModel{ID: id}` per discovered ID not
      already present, no entry marked as default). Verify: a test asserts
      the written `models.json` provider's `models` array contains the
      discovered IDs.
- [x] 4.6 Leave `lucinateHarness.Apply` unchanged: its `Connection` holds one
      model, so `DiscoveredModels` is not consulted there. Verify: a test
      confirms a gateway-only, no-Spinloop launch with `--harness lucinate`
      still succeeds, writing a connection with the gateway's URL and no
      `defaultModel` key.

## 5. Test coverage

- [x] 5.1 Add a test exercising `spinloop fleet harness` against a
      `fleet.yaml` with a `gateway:` section, no Spinloop file present, the
      token env var set, and a mock gateway serving both the proxy and
      `GET /v1/models`: assert it succeeds, writes an opencode config with
      the `openai-compatible` provider's `options.baseURL` set to the
      gateway's address, the discovered models listed, and no top-level
      default model. Repeat for `--harness pi`, asserting the same against
      `models.json`'s `models` array. Verify: both tests pass.
- [x] 5.2 Add/extend a test confirming the existing failure is unchanged when
      there is no Spinloop, no `-O`/argument given, and the fleet file names
      no gateway (or no fleet file at all). Verify: the test asserts the "a
      launch needs a Spinloop to know which model to route" error.
- [x] 5.3 Add a test confirming the no-Spinloop gateway case still fails
      correctly when the gateway's token is unset (existing "no token to
      reach the gateway" error, now via the fallback label from task 2.1).
      Verify: the test asserts the error names the fleet file, not an empty
      string.
- [x] 5.4 Add a test confirming a hand-written Spinloop with `PROVIDER
      openai-compatible` and no `MODEL`, routed at a gateway, also gets its
      model list populated the same way as the no-Spinloop case. Verify: the
      test passes.
- [x] 5.5 Run `go test ./... -cover` and confirm overall coverage stays
      >= 80%, per repository convention.

## 6. Documentation

- [x] 6.1 Run `/docs-update` (or otherwise check) for any README/docs
      mentioning `spinloop fleet harness`'s Spinloop requirement, and update
      if it documents the old always-fails behavior or omits the new model-
      list-from-gateway behavior.

## 7. Name the gateway-routed provider (found from user feedback)

- [x] 7.1 Add `Name string` (`yaml:"name"`) to `fleet.GatewayConfig`
      (`internal/fleet/config.go`) and a `Label()` method: the explicit name
      when set, else the URL's host, else the raw URL. Verify: a unit test
      covering all three cases, plus a test that `name` round-trips through
      `Load`/`GatewaySection`.
- [x] 7.2 Add `Label string` to `fleet.Choice` (`internal/fleet/select.go`);
      set it from `gw.Label()` in `routeThroughFleet`'s gateway branch
      (`cmd/spinloop/route.go`). Verify: `go build ./...` succeeds.
- [x] 7.3 Add `gatewayProviderKey`/`slugify` to `cmd/spinloop/main.go`:
      `"gateway-" + slugify(label)`, falling back to bare `"gateway"` when
      the label slugs to nothing. Verify: a table-driven unit test covering
      an address, a hand-picked name, mixed case/spacing, and an
      all-punctuation label.
- [x] 7.4 Replace `applySelection`'s `modelOptional bool` parameter with a
      `gatewayLabel string` (empty means the old `false`); in
      `applyRoutedSpinloop`, set it to `choice.Label` exactly when
      `sel.Model == "" && sel.Alias == ""` under a gateway choice — the same
      condition that already triggers `DiscoveredModels`. Verify:
      `go build ./...` succeeds and existing tests still pass with `""`
      passed at the three unrelated call sites.
- [x] 7.5 In `applySelection`, add an `else if gatewayLabel != ""` branch
      beside the existing `envName` rename block: `sel.Provider =
      gatewayProviderKey(gatewayLabel)`, `sel.DisplayName =
      catalog.RemoteProviderLabel(p.Name, gatewayLabel)`. Verify: the
      existing no-Spinloop-gateway tests (task 5.1's opencode/Pi cases, 5.4's
      hand-written-Spinloop case, and 4.6's lucinate case) updated to expect
      the derived key instead of the bare catalogue id, and a new test for an
      explicit `gateway.name` producing `"gateway-<name>"` and
      `"OpenAI-compatible (<name>)"`.
- [x] 7.6 Update `docs/commands/fleet.md` and `docs/commands/gateway.md` to
      document the `name` field and the resulting provider naming.
