## 1. Shared seed discovery

- [x] 1.1 Add `findSeedInstances(seedId?)` to `remote/lambda/shared/seed/` (a new `discovery.ts`), filtering managed instances by the seed tag and, when given, the seed-id tag, and export it. Verify `remote/lambda/seed/index.ts` imports it and drops its local `findSeeds`, and `pnpm test` in `remote/` passes.

## 2. The start Lambda's weights gate

- [x] 2.1 In `remote/lambda/start/index.ts`'s `wake()`, after the deploy-config and EIP/security-group checks and before the existing-instance lookup, check `weightsPresent(WEIGHTS_BUCKET, deployConfig)`; when true, proceed unchanged. Verify the existing start Lambda tests still pass with the check stubbed present.
- [x] 2.2 When the weights are absent, return the 503 `seeding` reply (`state`, `seedId` from `seedIdFor`, `retry_after_seconds`, a message naming `spinloop remote seed status <id>`) when a seed instance for that id is pending or running — stopped instances do not count. Verify a new vitest case asserts no `RunInstances` call is made and the reply carries the seed id.
- [x] 2.3 When no seed is in flight, enforce the in-flight cap (return the `seeding` reply with a cap message, no launch) and otherwise call `launchSeedInstance(buildSeedJob(deployConfig, infra, ''), infra)` with the environment's full deploy-config, returning the `seeding` reply. Verify new vitest cases: cap reached launches nothing; under the cap the launch receives the deploy-config's companions; the reply carries the launched seed id.
- [x] 2.4 Treat a seed launch failure (e.g. no capacity for the seed instance type) as the retryable `seeding` reply with the error in the message, not a fatal 502. Verify a new vitest case stubs the launch to throw and asserts a 503 `seeding` reply.

## 3. The Go start client

- [x] 3.1 In `internal/remote.Start`, special-case a 503 whose state is `seeding`: the progress line names the seed (`seedId` from the reply) instead of reading `instance <state>`. Verify a new test in `internal/remote` drives a seeding 503 and asserts the line names the seed and the loop continues until ready.
- [x] 3.2 When the context deadline expires, make the give-up error name the state last seen; when that state is seeding, carry the seed's follow command and the longer-`--timeout` hint. Verify a new test asserts the error text for a deadline reached during seeding, and that non-seeding deadlines keep today's message.

## 4. The fleet start phase

- [x] 4.1 In `internal/fleet/start_phase.go`, add a `PhaseSeeding` kind entered when `onState` reports `seeding`, rendered by `RenderPhase` as `seeding the weights (Xm Ys)` counting up from the first seeding reply. Verify `internal/fleet` tests cover the state mapping and the rendered line at two different `now` values.

## 5. CDK

- [x] 5.1 In `remote/lib/llm-stack.ts`, give the start function's environment `...seedEnv` and `MAX_CONCURRENT_SEEDS`, and its role the three grants it lacks for the seed launch: `ssm:GetParameter` on the stock AL2023 AMI parameter, `iam:PassRole` on the seed instance profile's role, and read on the weights bucket's `models/*` prefix. Verify `pnpm build` and `pnpm test` pass in `remote/` and `pnpm synth` produces a template whose StartFn policy includes the new statements.

## 6. Verification

- [x] 6.1 Run the Go suite with coverage (`go test ./... -cover`), `go vet ./...` and `gofmt -l .`, and confirm total coverage stays at or above 80% with no vet or formatting findings.
- [x] 6.2 Run the remote TypeScript suite (`pnpm test` in `remote/`) and confirm every new seed-gate, start-client and phase test passes.
- [x] 6.3 Walk the spec scenarios in `specs/endpoint-lifecycle/spec.md` against the new tests and confirm each one is exercised by at least one test case or explicitly noted in the PR description where it needs a live account.
