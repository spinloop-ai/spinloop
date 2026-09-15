## 1. The cloud-only facts

- [x] 1.1 Verify health already renders: a cloud node reporting unhealthy
      carries the same not-ready mark a daemon node does, through the `Ready`
      mapping and `servingText`. No code change expected — assert it with a
      test so the guarantee is held rather than assumed.
- [x] 1.2 Verify the address and the retention deadline are reachable without
      the removed command, and say where in the docs (design D2): the address
      from `remote env`, the deadline from `metrics` and the dashboard.

## 2. The verbs

- [x] 2.1 Add `cmd/spinloop/status.go`: a root-registered `status` taking
      `--env`/`--fleet`, resolving through `resolveFleetTarget` and rendering
      through `renderFleetStatus`. Verify `spinloop status --env <name>` and
      `spinloop status --fleet <path>` both render, and that naming both fails
      with the shared message.
- [x] 2.2 Add `cmd/spinloop/dashboard.go` the same way, over `dashModelFor`
      and `runDashProgram`. Verify `spinloop dashboard --env <name>` opens on
      one panel in a directory with no fleet file, and that a non-terminal
      invocation is refused as before.
- [x] 2.3 Register both at the root and carry over the completion the fleet
      spellings had (`--fleet` from files, `--env` from registered
      environments). Verify the completion test covers both new commands.

## 3. Removing the old spellings

- [x] 3.1 Delete `fleetStatusCmd` and `fleetDashboardCmd` and unregister them,
      keeping `renderFleetStatus`, `dashModelFor` and `runDashProgram` where
      they are. Verify `spinloop fleet --help` lists neither.
- [x] 3.2 Verify `spinloop remote status` is untouched and still works: it
      makes a second call for the version, applies a Spinloop's `ENV`, and
      renders the address and retention deadline, none of which a fan-out
      does. It goes with `metrics` and `logs`, not here.
- [x] 3.3 Signpost both moved spellings through the existing
      `movedSubcommands` (design D4). Verify each names its replacement, and
      that an unknown `fleet` subcommand still gets cobra's own error.

## 4. The no-target message

- [x] 4.1 Change `fleet.Resolve`'s failure to name the expected path,
      `--fleet <path>` and `--env <name>` (design D5). Verify a test asserts
      all three appear, and that the existing tests asserting on that message
      still pass or are updated to the fuller wording.
- [x] 4.2 Verify no implicit local target: a read verb with nothing resolvable
      fails as above even on a machine whose daemon is answering on its
      default port.

## 5. Consumers, docs and verification

- [x] 5.1 Update every invocation of the two moved spellings across `docs/`,
      `README.md` and `examples/` — including the CI-run
      `examples/fleet-docker/run-tests.sh` and `examples/gateway-docker/run-tests.sh`
      if they use them. Verify no consumer still invokes a removed spelling.
- [x] 5.2 Add `docs/commands/status.md` and `docs/commands/dashboard.md`, and
      point `fleet.md` and `remote.md` at them. Verify `docs/README.md`'s
      command table lists both.
- [x] 5.3 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
- [x] 5.4 Verify `metrics` and `logs` are untouched: no file under
      `cmd/spinloop/` that implements them changed except where a shared
      renderer moved, and `fleet metrics`/`fleet logs`/`remote metrics`/`remote
      logs` all still work.
