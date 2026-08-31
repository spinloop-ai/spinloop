# Tasks: install-spinloop-at-boot

## 1. Deploy-config contract (TypeScript)

- [x] 1.1 Add `spinloopVersion` to `DeployConfig` in `remote/lambda/shared/deploy-config.ts`, with an absent value normalised to `"latest"` by `parseDeployConfig`
- [x] 1.2 In `parseDeployConfig`, validate the pin's shape (character set `[0-9A-Za-z.-]`, no shell metacharacters — the value is interpolated into generated shell) and strip a leading `v`, so a `v1.26.1` pin stores as `1.26.1`
- [x] 1.3 Extend `remote/test/deploy-config.test.ts`: absent → `latest`, a bare pin round-trips, a `v`-prefixed pin is normalised, an empty or metacharacter-laden value is rejected, and a pre-pin config (field absent) parses unchanged

## 2. Boot-time install (TypeScript)

- [x] 2.1 Add a pure string-building `spinloopInstallStep(version: string)` to `remote/lambda/start/index.ts` (exported for tests) rendering the resolve-if-latest, idempotency-check, download, `checksums.txt` verify, atomic `install`, and `spinloop version` smoke-test from design.md D2/D3
- [x] 2.2 Emit the step from `buildInferenceUserData` after the CloudWatch agent starts and before the S3 weights sync, for both runners, so the binary is installed before `daemonBoot()` writes and enables the daemon's unit; the daemon's stored deploy config (`daemonDeployConfig`) does NOT gain the field — the pin is environment state, not engine config
- [x] 2.3 Extend `remote/test/start.test.ts`: the install step is present for both runners and precedes `spinloop-daemon.service` in the generated script, a pinned config renders the pin (no API call), an unpinned config resolves `latest` at boot, and the daemon's stored deploy-config JSON carries no `spinloopVersion`

## 3. Go client

- [x] 3.1 Add `SpinloopVersion string` with `json:"spinloopVersion,omitempty"` to `DeployConfig` in `internal/remote/remote.go`
- [x] 3.2 In `cmd/spinloop/remote.go`, add `--spinloop-version` to `remoteDeployCmd`, normalise the value in `runRemoteDeploy` (strip a leading `v`; treat empty and `latest` as no pin, so the field is omitted), set it on the `DeployConfig`, and print an `spinloop:` line (the pin, or `latest`) beside the runner and model in the plan, in both a real deploy and `--dry-run`
- [x] 3.3 Extend `internal/remote/remote_test.go`: the deploy request body carries `spinloopVersion` when set and omits it byte-for-byte when unset
- [x] 3.4 Extend `cmd/spinloop/remote_test.go`: the flag sets the field, `--spinloop-version=v1.2.3` and `--spinloop-version=1.2.3` both store `1.2.3`, empty and `latest` omit it, and the plan prints `spinloop: 1.2.3` / `spinloop: latest` accordingly

## 4. Remove spinloop from the bake

- [x] 4.1 In `remote/lib/image-stack.ts`: delete the spinloop block from `commonPreamble()`, the `SpinloopVersion` parameter from both component docs and `runnerBuilds`, and the synth-time empty-version guard; bump `RUNNER_VERSION` for both runners (component data changed, recipes are immutable per version)
- [x] 4.2 In `remote/lib/config.ts`: delete `spinloopVersion` from `LlmConfig`, `DEFAULTS`, and `loadConfig`, and delete `latestReleaseVersion()` with it
- [x] 4.3 In `.github/workflows/remote.yml`: delete the `-c spinloopVersion=0.0.0-ci` smoke-synth placeholder and its now-stale comment
- [x] 4.4 Update `remote/test/stack.test.ts`: drop the `spinloopVersion` context pin, the git-tag-default and empty-version-refusal tests, and assert the synthesised image recipe carries no `SpinloopVersion` parameter and the bake component contains no spinloop download/install

## 5. Documentation

- [x] 5.1 Update `remote/docs/architecture.md`: the "Image stack" section (the AMI no longer carries spinloop; the boot installs it) and the idle-check compatibility note that currently says AMIs must be re-baked before the control plane deploys — with the boot install, a fresh launch always carries a current spinloop
- [x] 5.2 Update `docs/commands/remote.md`'s deploy section: the `--spinloop-version` flag, that the default is `latest` resolved at boot, and that a pin takes effect on the next fresh launch (a re-wake keeps the installed version)

## 6. Verification and rollout

- [x] 6.1 Go side: `gofmt -l .`, `go vet ./...`, `go test ./... -cover` (total coverage stays >= 80%)
- [x] 6.2 remote/ side: `pnpm test` and `npx cdk synth -c allowedCidr=203.0.113.7/32 -c runner=llamacpp --quiet` (no `spinloopVersion` context) both pass
- [x] 6.3 Roll out per design.md's Migration Plan: deploy the control plane (`spinloop remote bootstrap`) against the current spinloop-baked AMIs, verify a fresh launch per runner (status shows a version; a `--spinloop-version`-pinned deploy installs exactly the pin), then `cdk deploy` the image stack and `pnpm bake` both spinloop-less runners
