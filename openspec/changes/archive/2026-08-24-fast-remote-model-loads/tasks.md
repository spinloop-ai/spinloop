## 1. Daemon pre-warm (Go)

_Superseded: the live check in 3.3 showed the pre-warm cannot get ahead of
the engine's faults, so the feature was removed — see section 4. These items
were completed as built._

- [x] 1.1 Implement the page-cache pre-warm (`internal/daemon/prewarm.go`: `prewarmModel`/`prewarmPath`/`modelFiles` — sequential buffered read of the model path, file or directory; background; silent no-op on a missing path)
- [x] 1.2 Unit-test the pre-warm (model file selection, read-through to the file, never blocks, `StartEngine` wiring)
- [x] 1.3 Make the pre-warm opt-in: `Daemon.Prewarm` nil means never; `spinloop daemon --prewarm` wires `prewarmModel` in; the `serve --api` and fleet constructions leave it nil
- [x] 1.4 Add `prewarm` (`*bool`, tri-state) to `internal/remote`'s `DeployConfig`; `StartEngine` honours the per-start choice under the ceiling rule (daemon option AND config field)
- [x] 1.5 Update the `StartEngine` wiring tests to the ceiling model: option off never pre-warms; option on with a start disabled skips; option on with no choice pre-warms
- [x] 1.6 Update the daemon's OpenAPI description and its contract test for the deploy-config's new field
- [x] 1.7 Update the AGENTS.md daemon entry for the option and the ceiling rule

## 2. Control plane (remote/)

- [x] 2.1 Launch with a provisioned gp3 root: block device mapping at the AMI's own root size (from `findLatestAmi`), 1,000 MiB/s of throughput and the 4,000 IOPS that ceiling requires; seed instance unprovisioned
- [x] 2.2 Tests for the launch's block device mapping and the fresh-launch wiring (`launch-volume.test.ts`, `start-launch.test.ts`)
- [x] 2.3 `daemonBoot`: drop the start loop — the boot writes the deploy config, enables the daemon, and stops; the daemon's first answer is the boot's completion signal
- [x] 2.4 Start Lambda: after SSM is online, wait until the instance's daemon answers its control API, on every path, before issuing the start
- [x] 2.5 Start Lambda: issue the daemon's start with the rendered deploy config as its body, on a fresh launch and a re-wake alike; the SSM start command carries the body
- [x] 2.6 Tests for the daemon-ready wait and the body-carrying start (Lambda flows, the daemon unit render)
- [x] 2.7 Docs: README (who starts the engine), costs.md (provisioned throughput and IOPS), the `gracePeriodMinutes` comment, and the release-order note

## 3. Verification

- [x] 3.1 `go vet ./...`, `go test ./...` (total coverage stays >= 80%), `gofmt`
- [x] 3.2 `remote/`: `pnpm build`, `pnpm test`, the cloud-identifier guard
- [x] 3.3 End-to-end on a real environment (dev-3, 2026-08-23): a cold wake, a re-wake, and a fresh cold launch measured the volume and the load. The pre-warm's own reads went flat seconds in while the engine's faults owned the 4,000 IOPS, the box's 32 GB cannot cache a ~30 GB model, and the load ran ~115 s with the pre-warm on and no faster without it — the pre-warm was removed (section 4) and the provisioned volume kept

## 4. Pre-warm removal (after the live check)

- [x] 4.1 Go: remove the daemon pre-warm (`internal/daemon/prewarm.go` and its tests, the `Daemon.Prewarm` wiring, `spinloop daemon --prewarm`), `DeployConfig`'s tri-state field, and the start/restart signatures it threaded through; update the OpenAPI description and its contract test
- [x] 4.2 Control plane: remove the pre-warm parameter from the start Lambda, the deploy-config JSON's property, the daemon unit's `--prewarm`, and the `spinloop remote start`/`restart` flags with their query parameter
- [x] 4.3 Docs: AGENTS.md (a trap recording why a pre-warm cannot beat the engine's faults), README and costs.md (first boot, re-wake, the volume's cost), the OpenAPI spec
- [x] 4.4 `go vet`, `go test` (coverage stays >= 80%), `pnpm build`, and `pnpm test` all green after the removal
