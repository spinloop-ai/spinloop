## 1. A fleet of one

- [x] 1.1 Add a constructor in `internal/fleet` that returns a `*Config`
      holding one cloud node named by a registered environment, running the
      same validation `Load` runs, with `Path` and `Dir` left empty. Verify a
      unit test builds one and `NewNode` yields a working cloud node for it.
- [x] 1.2 Verify the constructor rejects a name that is not a plain
      identifier and a name with no registered configuration, each with the
      message the `remote` subcommands give, before any node is contacted.
      Verify with table tests over both cases.
- [x] 1.3 Verify a one-node config resolves no token reference and reads no
      adjacent `.env` — a test with a `.env` in the working directory holding
      a variable the node would otherwise want, asserting it is not consulted.

## 2. One target resolver

- [x] 2.1 Add the resolver taking what the flags say (`--env`, `--fleet`) and
      returning the fleet to act on: the environment's fleet of one, the named
      file, or the working directory's `fleet.yaml`. Verify with table tests
      over all three paths plus the missing-file case.
- [x] 2.2 Make the resolver fail when `--env` and `--fleet` are both given,
      naming both, reusing the launch path's existing sentence verbatim.
      Verify the error names both values, and that `--env` beside a
      *directory's* `fleet.yaml` is not a conflict (design D4).
- [x] 2.3 Move the launch path's exclusivity check in
      `cmd/spinloop/main.go` onto the resolver so one rule is enforced from
      one place. Verify the existing launch tests for that error still pass
      unchanged.

## 3. The fleet commands

- [x] 3.1 Add `--env` (long form only, per design D5) to `fleet status`,
      `fleet metrics`, `fleet start`, `fleet stop`, `fleet deploy` and
      `fleet route` — not `fleet harness`, per design D6 — and replace each
      `fleet.Resolve` call with the resolver. Verify each command's existing
      tests pass and `spinloop fleet status --env <name>` acts on that
      environment.
- [x] 3.2 Add `--env` to `fleet logs` and `fleet dashboard` the same way,
      keeping `-f` as `--follow` on `logs`. Verify `fleet dashboard --env
      <name>` opens on one environment in a directory with no `fleet.yaml`.
- [x] 3.3 Register completion for the new flag from the same source
      `remote`'s `--env` completes from, so both offer the registered
      environments. Verify the completion test covers the new flag.
- [x] 3.4 Verify anything reporting which fleet it acted on names the
      environment rather than an empty path — check `fleet route`'s
      "Routing through …" line and the fan-out's error paths.

## 4. Verification and docs

- [x] 4.1 Verify a `--env` target renders identically to the same environment
      listed as a one-node fleet file: a test asserting both paths produce the
      same status output.
- [x] 4.2 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
- [x] 4.3 Verify the `remote` group is untouched: no file under
      `cmd/spinloop/remote*.go` changed except where the shared resolver
      replaced a duplicated check, and `remote status --env x` prints what it
      printed before.
- [x] 4.4 Document the flag in `docs/commands/fleet.md`: what it targets, that
      it needs no fleet file, that it cannot be combined with `--fleet`, and
      that a fleet of one carries no fleet-wide settings.
