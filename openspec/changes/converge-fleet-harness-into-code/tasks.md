## 1. The launch absorbs the gateway branch

- [ ] 1.1 Replace the launch's `--fleet needs a Spinloop` refusal in
      `cmd/spinloop/code.go` with the gateway check `runFleetHarness` performs:
      resolve the fleet, and where it names a gateway proceed with the gateway
      provider selection instead of failing. Verify `spinloop code --fleet
      <path>` launches in a directory holding no Spinloop when that file names
      a gateway.
- [ ] 1.2 Verify the refusal still stands where the fleet names no gateway,
      with its existing wording — a launch needs a Spinloop to know which model
      to route. Verify with a test over both fleet files.
- [ ] 1.3 Move the gateway helpers `runFleetHarness` calls (the gateway
      provider id, the `GET /v1/models` fetch, the provider label) to the
      launch path beside their only remaining caller, per design D4. Verify
      the package builds with no exported surface added.
- [ ] 1.4 Verify the moved behaviours are unchanged: the provider is named
      "Gateway (<name-or-address>)", the model list comes from the live
      gateway, and a failed model query warns and launches with an empty list
      rather than failing. Port the existing `fleet_harness_test.go` cases for
      each onto the launch.
- [ ] 1.5 Verify a case neither command has run before: trailing arguments
      forwarded to the agent alongside a gateway launch, which `fleet harness`
      dropped and the launch forwards (design risk note).

## 2. Removing the command

- [ ] 2.1 Delete `fleetHarnessCmd`, `runFleetHarness` and `cmdFleetHarness`
      from `cmd/spinloop/fleet.go`, and unregister the subcommand in
      `cmd/spinloop/commands.go`. Verify `spinloop fleet --help` no longer
      lists it.
- [ ] 2.2 Signpost the moved spelling on the `fleet` group, per design D3, so
      `spinloop fleet harness` fails with `"fleet harness" moved: run spinloop
      code --fleet <path>` rather than an unknown-command error. Verify with a
      test asserting the message names the replacement.
- [ ] 2.3 Delete `cmd/spinloop/fleet_harness_test.go`, having ported its
      gateway cases in 1.4. Verify no test references the removed functions.

## 3. Consumers

- [ ] 3.1 Update `examples/gateway-docker/run-tests.sh` (two invocations, plus
      the comment above them) to `spinloop code --fleet <path>`. Verify the
      example's test run passes — it is CI-run, so this is the change's
      end-to-end check.
- [ ] 3.2 Update `examples/gateway-docker/README.md` and any other example
      naming the command. Verify no example still invokes `fleet harness`.
- [ ] 3.3 Update `docs/commands/fleet.md` (its "Launching the harness" section
      and the `--env` note added by the previous change),
      `docs/commands/harness.md`, `docs/commands/gateway.md` and
      `docs/commands/code.md` to name the replacement. Verify no doc still
      documents `fleet harness` as a command.

## 4. Verification

- [ ] 4.1 Verify a launch that names its Spinloop explicitly no longer picks up
      a working-directory `fleet.yaml` on this path either (design D2) — the
      one deliberate behaviour change — and that passing `--fleet` restores it.
- [ ] 4.2 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
- [ ] 4.3 Verify the tree has one launch command: grep the command tree for a
      second registration and confirm `code`, `harness open` and nothing else
      reach `launchAgent`.
