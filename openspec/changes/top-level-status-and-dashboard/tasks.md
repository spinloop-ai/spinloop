## 1. Carrying the cloud-only facts

- [ ] 1.1 Add the endpoint's health and base URL to `fleet.NodeResult`, filled
      by `statusFromRemote` from the reply it already receives and left empty
      by a daemon node (design D2). Verify a unit test over both node kinds:
      the cloud node carries both, the daemon node neither.
- [ ] 1.2 Render them in the status table when present, omitted when not.
      Verify the same environment read through a fleet file shows what
      `remote status` shows today for health and address.

## 2. The verbs

- [ ] 2.1 Add `cmd/spinloop/status.go`: a root-registered `status` taking
      `--env`/`--fleet`, resolving through `resolveFleetTarget` and rendering
      through `renderFleetStatus`. Verify `spinloop status --env <name>` and
      `spinloop status --fleet <path>` both render, and that naming both fails
      with the shared message.
- [ ] 2.2 Add `cmd/spinloop/dashboard.go` the same way, over `dashModelFor`
      and `runDashProgram`. Verify `spinloop dashboard --env <name>` opens on
      one panel in a directory with no fleet file, and that a non-terminal
      invocation is refused as before.
- [ ] 2.3 Register both at the root and carry over the completion the fleet
      spellings had (`--fleet` from files, `--env` from registered
      environments). Verify the completion test covers both new commands.

## 3. Removing the old spellings

- [ ] 3.1 Delete `fleetStatusCmd` and `fleetDashboardCmd` and unregister them,
      keeping `renderFleetStatus`, `dashModelFor` and `runDashProgram` where
      they are. Verify `spinloop fleet --help` lists neither.
- [ ] 3.2 Delete `remoteStatusCmd` and `runRemoteStatus` and unregister them.
      Verify `spinloop remote --help` no longer lists `status`, and that
      nothing else referenced the removed functions.
- [ ] 3.3 Signpost all three moved spellings (design D4): `fleet status` and
      `fleet dashboard` through the existing `movedSubcommands`, and
      `remote status` by giving the `remote` group the same `Args: groupArgs`
      the fleet group has. Verify each names its replacement, and that an
      unknown subcommand in either group still gets cobra's own error.

## 4. The no-target message

- [ ] 4.1 Change `fleet.Resolve`'s failure to name the expected path,
      `--fleet <path>` and `--env <name>` (design D5). Verify a test asserts
      all three appear, and that the existing tests asserting on that message
      still pass or are updated to the fuller wording.
- [ ] 4.2 Verify no implicit local target: a read verb with nothing resolvable
      fails as above even on a machine whose daemon is answering on its
      default port.

## 5. Consumers, docs and verification

- [ ] 5.1 Update every invocation of the three moved spellings across `docs/`,
      `README.md` and `examples/` — including the CI-run
      `examples/fleet-docker/run-tests.sh` and `examples/gateway-docker/run-tests.sh`
      if they use them. Verify no consumer still invokes a removed spelling.
- [ ] 5.2 Add `docs/commands/status.md` and `docs/commands/dashboard.md`, and
      point `fleet.md` and `remote.md` at them. Verify `docs/README.md`'s
      command table lists both.
- [ ] 5.3 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
- [ ] 5.4 Verify `metrics` and `logs` are untouched: no file under
      `cmd/spinloop/` that implements them changed except where a shared
      renderer moved, and `fleet metrics`/`fleet logs`/`remote metrics`/`remote
      logs` all still work.
