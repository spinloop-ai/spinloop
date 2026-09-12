# Tasks

## 1. Spinloop grammar

- [x] 1.1 `internal/spinloop`: drop the `Fleet` field, the `kwFleet` parse
      case, the REMOTE/FLEET exclusivity check, `FleetIsEndpoint`, and the
      `FLEET` line in `Format`; `FLEET` now falls into the unknown-keyword
      error
- [x] 1.2 `internal/spinloop` tests: remove the FLEET parse/format cases and
      the REMOTE+FLEET conflict case; add a case that a `FLEET` line fails as
      an unknown keyword naming the accepted keywords

## 2. Routing discovery

- [x] 2.1 `cmd/spinloop`: compute whether the Spinloop was named explicitly
      (positional, `--spinloop` value, or `SPINLOOP_ALIAS`) and thread it
      through `applyBeforeLaunch`, `routeThroughFleet`, and the `fleet route`
      and `fleet harness` paths
- [x] 2.2 `routeThroughFleet`: the fleet file comes from `--fleet`/`-f`, else
      `./fleet.yaml` in the working directory when the Spinloop was not
      explicitly named, else no routing; delete the endpoint branch and
      `fleetTarget`'s `sel.Fleet`
- [x] 2.3 `spinloop fleet route`: fleet file from `--fleet`/`-f`, else
      `./fleet.yaml` for a not-explicitly-named Spinloop; with none in force,
      fail naming `--fleet`; delete the endpoint reporting branch
- [x] 2.4 `spinloop fleet harness`: drop the Spinloop-`FLEET` tier — the
      fleet file is `--fleet`/`-f` or the `fleet.yaml` beside the command

## 3. Gateway command output

- [x] 3.1 `cmd/spinloop/gateway.go`: the printed address is named in a fleet
      file's `gateway` section, not a Spinloop's `FLEET` (usage text included)

## 4. Tests

- [x] 4.1 Update the FLEET-based launch tests in `cmd/spinloop` to the
      directory convention or `--fleet`; remove the endpoint-`FLEET` tests
- [x] 4.2 New tests: an explicitly named Spinloop in a directory holding a
      `fleet.yaml` does not route; `SPINLOOP_ALIAS` does not trigger the
      lookup; a valueless `--spinloop` beside a `fleet.yaml` routes, while a
      bare `spinloop harness` (which wears nothing) does not; `--fleet` routes
      an explicit Spinloop
- [x] 4.3 `fleet route` and `fleet harness` tests: no-`FLEET` resolution and
      the no-fleet-file failure naming `--fleet`

## 5. `add-fleet-gateway` delta trim

- [x] 5.1 Delete `openspec/changes/add-fleet-gateway/specs/spinloop-files/`
- [x] 5.2 Remove the two endpoint-`FLEET` requirements from
      `openspec/changes/add-fleet-gateway/specs/fleet-routing/spec.md`
- [x] 5.3 Reword the `fleet-gateway` requirement and scenario so the printed
      address is named in a fleet file's `gateway` section
- [x] 5.4 `openspec validate` passes for both changes

## 6. Docs

- [x] 6.1 `docs/spinloop-file.md`: drop the `FLEET` section and the
      REMOTE/FLEET exclusivity; the keyword table loses `FLEET`; point readers
      at the discovery rules
- [x] 6.2 `docs/commands/harness.md` and `docs/commands/fleet.md`: replace
      "overrides the Spinloop's `FLEET`" with the discovery rules; the
      `fleet route` no-fleet error now names `--fleet`
- [x] 6.3 `docs/commands/gateway.md`: the address goes in a fleet file's
      `gateway` section
- [x] 6.4 Root `README.md`: the Spinloop example and keyword list lose
      `FLEET`; the routing paragraph describes the flag and the directory
      convention

## 7. Examples

- [x] 7.1 `examples/fleet-local`: drop the `FLEET` line from the Spinloop;
      README wording for why routing happens
- [x] 7.2 `examples/fleet-docker`: drop `FLEET` from `client/Spinloop`; the
      harness invocation in `run-tests.sh` passes `--fleet`; README table and
      commands updated
- [x] 7.3 `examples/gateway-docker`: verify its run against the new
      resolution and adjust only if it breaks

## 8. Verify

- [x] 8.1 `go build ./... && go vet ./... && gofmt -l internal/ cmd/` clean,
      `go test ./... -cover` green with the mean at or above the bar, and the
      `remote/` pnpm suite green
- [x] 8.2 `examples/fleet-docker/run-tests.sh` and
      `examples/gateway-docker/run-tests.sh` green end to end
