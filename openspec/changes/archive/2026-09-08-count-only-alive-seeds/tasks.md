## 1. The shared alive judgement

- [x] 1.1 Move the gate's `seedAlive` into `remote/lambda/shared/seed/discovery.ts` next to `findSeedInstances`, and use it from the start Lambda's gate, the state join in `remote/lambda/shared/seed/status.ts`, and the seed Lambda's join and cap. Verify `pnpm build` and `pnpm test` pass in `remote/`.

## 2. The seed surface counts alive seeds

- [x] 2.1 In `remote/lambda/seed/index.ts`'s start path, filter the join and the cap to pending and running instances, so a stopped seed is neither joined nor counted. Verify new vitest cases assert a stopped seed is re-seeded rather than joined, and that a stopped seed does not fill a cap slot.

## 3. The seed control surface

- [x] 3.1 Add handler tests for the seed Lambda's start, status, list and stop paths (the file carried no tests), through the public handler with the AWS calls stubbed. Verify `pnpm test` in `remote/` passes.

## 4. The Go start client and the phase stream

- [x] 4.1 Verify a new test in `internal/remote` drives a `seeding` 503 that carries no seed id and asserts the progress line and the give-up error stay generic, naming no seed.
- [x] 4.2 Verify a new test in `internal/fleet` drives a seeding reply followed by a `starting` reply and asserts the phase stream enters a boot whose clock starts at the handoff.

## 5. Verification

- [x] 5.1 Run the Go suite with coverage (`go test ./... -cover`), `go vet ./...` and `gofmt -l .`, and confirm total coverage stays at or above 80% with no vet or formatting findings.
- [x] 5.2 Run the remote TypeScript suite (`pnpm test` in `remote/`) and confirm every new test passes.
- [x] 5.3 Validate the change with `openspec change validate count-only-alive-seeds` and walk the new weight-seeding scenarios against the tests.
