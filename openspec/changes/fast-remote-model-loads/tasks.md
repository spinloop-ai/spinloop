## 1. Daemon pre-warm (Go)

- [x] 1.1 Implement the page-cache pre-warm (`internal/daemon/prewarm.go`: `prewarmModel`/`prewarmPath`/`modelFiles` — sequential buffered read of the model path, file or directory; background; silent no-op on a missing path)
- [x] 1.2 Unit-test the pre-warm (model file selection, read-through to the file, never blocks, `StartEngine` wiring)
- [x] 1.3 Make the pre-warm opt-in: `Daemon.Prewarm` nil means never; `outfit daemon --prewarm` wires `prewarmModel` in; the `serve --api` and fleet constructions leave it nil
- [x] 1.4 Add `prewarm` (`*bool`, tri-state) to `internal/remote`'s `DeployConfig`; `StartEngine` honours the per-start choice under the ceiling rule (daemon option AND config field)
- [x] 1.5 Update the `StartEngine` wiring tests to the ceiling model: option off never pre-warms; option on with a start disabled skips; option on with no choice pre-warms
- [x] 1.6 Update the daemon's OpenAPI description and its contract test for the deploy-config's new field
- [x] 1.7 Update the AGENTS.md daemon entry for the option and the ceiling rule

## 2. Control plane (remote/)

- [x] 2.1 Launch with a provisioned gp3 root: block device mapping at the AMI's own root size (from `findLatestAmi`) and 1,000 MiB/s throughput; seed instance unprovisioned
- [x] 2.2 Tests for the launch's block device mapping and the fresh-launch wiring (`launch-volume.test.ts`, `start-launch.test.ts`)
- [x] 2.3 `daemonBoot`: the daemon unit's `ExecStart` passes `--prewarm`; drop the start loop — the boot writes the deploy config, enables the daemon, and stops; the daemon's first answer is the boot's completion signal
- [x] 2.4 Start Lambda: after SSM is online, wait until the instance's daemon answers its control API, on every path, before issuing the start
- [x] 2.5 Start Lambda: issue the daemon's start with the rendered deploy config as its body — the pre-warm resolved to the operator's explicit choice, else the cloud default — on a fresh launch and a re-wake alike; the SSM start command carries the body
- [x] 2.6 Plumb the choice: `outfit remote start` and `restart` flags → `internal/remote.Start`'s query parameter → the Lambda's pre-warm parameter
- [x] 2.7 Add the tri-state `prewarm` to the shared deploy-config TS type
- [x] 2.8 Tests for the daemon-ready wait, the body-carrying start, and the pre-warm plumbing (Lambda flows, the daemon unit render, the CLI flag)
- [x] 2.9 Docs: README (who starts the engine, the pre-warm default and how to skip it), costs.md (provisioned throughput), the `gracePeriodMinutes` comment, and the release-order note (release → bake → deploy)

## 3. Verification

- [x] 3.1 `go vet ./...`, `go test ./...` (total coverage stays >= 80%), `gofmt`
- [x] 3.2 `remote/`: `pnpm build`, `pnpm test`, the cloud-identifier guard
- [ ] 3.3 End-to-end on a real environment: a cold wake pre-warms by default, `--no-prewarm` skips it, a re-wake pre-warms, and a fleet node and a local daemon never do
