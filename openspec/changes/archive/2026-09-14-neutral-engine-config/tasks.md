## 1. The new package

- [x] 1.1 Create `internal/inference/inference.go` and move `DeployConfig` into it
      verbatim from `internal/remote/remote.go` — struct, doc comment and every
      field comment unchanged, only the package clause differing. Verify
      `go build ./internal/inference` succeeds and the file imports nothing from
      `internal/`.
- [x] 1.2 Point `internal/remote` at the new home: drop the local definition,
      import `internal/inference`, and update `Deploy` and its embedded request
      struct. Verify `go test ./internal/remote/...` passes unchanged, and that
      `IsInstanceType` is still in `internal/remote` per design D2.

## 2. Breaking the daemon's cloud dependency

- [x] 2.1 Update `internal/daemon` (`daemon.go`, `api.go`) to take
      `DeployConfig` from `internal/inference` — the stored config, the
      `BuildArgv`/`EngineKeyArgs`/`ValidateConfig` hook signatures, and the
      start request body. Verify `go test ./internal/daemon/...` passes with no
      test changed except its import block.
- [x] 2.2 Change `daemon.StateDir` to call `config.Dir` directly instead of
      `remote.ConfigHome`, per design D4. Verify the existing `StateDir` tests
      pass and the resolved path is byte-identical.
- [x] 2.3 Verify `internal/daemon` no longer references `internal/remote` at
      all: `go list -deps ./internal/daemon | grep spinloop/internal/remote`
      returns nothing, for the package and its tests.

## 3. The remaining callers

- [x] 3.1 Update `internal/fleet` (`node.go`, `client.go`, `wake.go`,
      `remote_node.go`) to reference `inference.DeployConfig`, including the
      `Node.StartWith` and `ConfigFor` signatures. Verify
      `go test ./internal/fleet/...` passes.
- [x] 3.2 Update `internal/gateway` and `internal/orchestrator` to the new
      package name. Verify `go test ./internal/gateway/... ./internal/orchestrator/...`
      passes.
- [x] 3.3 Update `cmd/spinloop` (`remote.go`, `serve_daemon.go`, `gateway.go`)
      and the test files that construct a deploy config. Verify
      `go test ./cmd/...` passes.

## 4. Verification and docs

- [x] 4.1 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, and confirm total coverage is unchanged from the
      pre-change run and still >= 80%.
- [x] 4.2 Confirm the diff contains no logic change: every hunk is a package
      clause, an import, or a type reference. Nothing in `remote/` (the
      TypeScript control plane), `docs/openapi.yaml`, or any on-disk format is
      touched.
- [x] 4.3 Add `internal/inference` to the package list in `AGENTS.md`, and record
      the layering rule — `internal/daemon` depends on no cloud package, and
      the neutral package imports nothing of ours — in `docs/internals.md`.
