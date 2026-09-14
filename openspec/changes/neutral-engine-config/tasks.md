## 1. The new package

- [ ] 1.1 Create `internal/engine/engine.go` and move `DeployConfig` into it
      verbatim from `internal/remote/remote.go` — struct, doc comment and every
      field comment unchanged, only the package clause differing. Verify
      `go build ./internal/engine` succeeds and the file imports nothing from
      `internal/`.
- [ ] 1.2 Point `internal/remote` at the new home: drop the local definition,
      import `internal/engine`, and update `Deploy` and its embedded request
      struct. Verify `go test ./internal/remote/...` passes unchanged, and that
      `IsInstanceType` is still in `internal/remote` per design D2.

## 2. Breaking the daemon's cloud dependency

- [ ] 2.1 Update `internal/daemon` (`daemon.go`, `api.go`) to take
      `DeployConfig` from `internal/engine` — the stored config, the
      `BuildArgv`/`EngineKeyArgs`/`ValidateConfig` hook signatures, and the
      start request body. Verify `go test ./internal/daemon/...` passes with no
      test changed except its import block.
- [ ] 2.2 Change `daemon.StateDir` to call `config.Dir` directly instead of
      `remote.ConfigHome`, per design D4. Verify the existing `StateDir` tests
      pass and the resolved path is byte-identical.
- [ ] 2.3 Verify `internal/daemon` no longer references `internal/remote` at
      all: `go list -deps ./internal/daemon | grep spinloop/internal/remote`
      returns nothing, for the package and its tests.

## 3. The remaining callers

- [ ] 3.1 Update `internal/fleet` (`node.go`, `client.go`, `wake.go`,
      `remote_node.go`) to reference `engine.DeployConfig`, including the
      `Node.StartWith` and `ConfigFor` signatures. Verify
      `go test ./internal/fleet/...` passes.
- [ ] 3.2 Update `internal/gateway` and `internal/orchestrator` to the new
      package name. Verify `go test ./internal/gateway/... ./internal/orchestrator/...`
      passes.
- [ ] 3.3 Update `cmd/spinloop` (`remote.go`, `serve_daemon.go`, `gateway.go`)
      and the test files that construct a deploy config. Verify
      `go test ./cmd/...` passes.

## 4. Verification and docs

- [ ] 4.1 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, and confirm total coverage is unchanged from the
      pre-change run and still >= 80%.
- [ ] 4.2 Confirm the diff contains no logic change: every hunk is a package
      clause, an import, or a type reference. Nothing in `remote/` (the
      TypeScript control plane), `docs/openapi.yaml`, or any on-disk format is
      touched.
- [ ] 4.3 Add `internal/engine` to the package list in `AGENTS.md`, and record
      the layering rule — `internal/daemon` depends on no cloud package, and
      the neutral package imports nothing of ours — in `docs/internals.md`.
