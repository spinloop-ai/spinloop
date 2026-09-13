## 1. env Lambda reports the deploy-config

- [x] 1.1 In `remote/lambda/env/index.ts`, read the deploy-config via `readDeployConfig(deployConfigParam(env))` (the same helpers `remote/lambda/start/index.ts` already imports), guarding the read so a missing or unparsable parameter degrades to omitting the fields rather than failing the response — verify with a new `remote/test/env-deploy-config.test.ts` (mirroring `remote/test/stats-relay.test.ts`'s mocking of the SSM read) covering both the present and the missing/unparsable case.
- [x] 1.2 Add `deployed`, `runner`, `modelId`, `servedName`, `contextSize` to the Lambda's JSON reply when the deploy-config is present, using the same field names `start`/`stats` already emit — verify the existing `remote/test/env-api-key.test.ts` still passes unchanged (the new fields are additive) and the new test from 1.1 asserts their presence/absence.
- [x] 1.3 Run `pnpm test` (or the configured `vitest run`) in `remote/` and confirm the whole suite passes.
- [x] 1.4 Grant the env Lambda's role `ssm:GetParameter` on the deploy-config parameter in `remote/lib/llm-stack.ts` (`readEnvParamsStatement`, the same read-only grant `stopFn` already has) — found missing during live testing: without it, `readDeployConfig` throws `AccessDenied`, caught by 1.1's best-effort try/catch and silently reported as "nothing deployed" on an environment that is actually running. Verify with a new `remote/test/stack.test.ts` assertion that the env Lambda's policy carries a read-only `ssm:GetParameter` grant on the `cloud-vm-llm` parameter path.

## 2. Go client: runner → provider mapping

- [x] 2.1 In `cmd/spinloop/remote.go`, add a small helper next to `runnerFor` that maps a `Runner` string back to a catalogue provider name (the reverse of `runnerFor`'s identity mapping for `llamacpp`/`vllm`), returning an error for anything else — verify with a table-driven unit test covering both known runners and an unrecognised one.

## 3. CLI: auto-configure from a fetched environment

- [x] 3.1 In `cmd/spinloop/main.go`, extend `applyRoutedSpinloop` (or its caller) so that when no Spinloop was read and `route.envName != ""`, it fetches the environment's response first and, when the response carries a deploy-config (`Deployed` true with `Runner`/`ServedName` set), synthesises a `spinloop.Selection{Provider: <mapped runner>, Alias: resp.ServedName, Context: resp.ContextSize}` and passes it through the existing `applySelection` call — verify with a unit test exercising this path directly (in the style of the existing `applyBeforeLaunch`/`applyRoutedSpinloop` tests in `cmd/spinloop/harness_remote_test.go`), asserting the written harness config matches what an equivalent Spinloop would produce.
- [x] 3.2 In the same path, when no Spinloop was read, `route.envName != ""`, and the fetched response carries no deploy-config (nothing deployed, or an `env` Lambda predating this change), fail before writing any harness config with an error naming the environment and both remediations (`spinloop remote deploy <spinloop> --env <name>`, and `spinloop remote bootstrap` for a stale control plane) — verify with a unit test asserting the error text and that no config file is written.
- [x] 3.3 In `cmd/spinloop/commands.go`'s `harnessCmd` `RunE`, widen the gate that currently calls `applyBeforeLaunch` only `if spinloopPath.set` to also enter it when `route.envName != ""`, so a bare `spinloop harness --env <name>` reaches the new path from 3.1/3.2 instead of silently launching unconfigured — verify with an end-to-end `cmdHarness` test (stubbing the harness binary and the env Lambda's HTTP response, following the pattern in `cmd/spinloop/harness_remote_test.go`) that a bare `--env` launch writes the expected config and forwards trailing args, and that a Spinloop applied alongside `--env` still wins over the deploy-config (regression test for the existing precedence).
- [x] 3.4 Confirm every existing scenario in `openspec/specs/harness-remote-env/spec.md` and `openspec/specs/remote-env/spec.md` (unmodified by this change) still passes — run `go test ./cmd/spinloop/... ./internal/remote/...` and confirm no regressions.

## 4. Docs

- [x] 4.1 Update `docs/commands/harness.md` to document the bare `--env <name>` auto-configure flow, its precedence against an applied Spinloop, and the "nothing deployed" / "update the control plane" errors.
- [x] 4.2 Update `docs/commands/remote.md` to note that `spinloop remote env` (and the `env` Lambda generally) now reports what is deployed, when anything is.

## 5. End-to-end verification

- [x] 5.1 Run `go build ./...`, `go vet ./...`, `gofmt -l` over changed Go files, and `go test ./...` (target ≥80% coverage per project convention) — all green.
- [x] 5.2 Manually verify against a real (or locally stubbed) deployed environment: `spinloop remote deploy <spinloop> --env dev-test` from one config, then `spinloop harness --env dev-test <args>` with no Spinloop applied, confirming the harness launches configured and the trailing args reach it.
- [x] 5.3 Manually verify the failure path: `spinloop harness --env <registered-but-undeployed-name>` with no Spinloop fails with the documented error and writes no config.
