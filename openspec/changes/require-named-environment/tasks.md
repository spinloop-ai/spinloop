## 1. One resolution rule

- [x] 1.1 Give `LoadConfigFile` the environment name, and have it return a
      config carrying that name as the identifier where the file is absent but
      the `SPINLOOP_REMOTE_*` overrides are complete (design D3). Verify a test
      that `--env ci` with no file and complete overrides yields a config whose
      environment is `ci`.
- [x] 1.2 Delete `LoadDefault`, `LoadConfig` and `ConfigPath` (design D4).
      Verify the tree builds — a missed caller is a compile error, not a
      silent change.
- [x] 1.3 Verify no path outside the registry is consulted for any name: a
      test that a file at the superseded path is not read for `--env default`.

## 2. Requiring the flag

- [x] 2.1 Make `--env` required on `start`, `stop`, `pause`, `restart`,
      `status`, `metrics`, `logs`, `env` and `keep` (design D1), and drop
      `resolveRemoteConfig`'s fallback branch so it takes a name it can rely
      on. Verify each subcommand fails without the flag and works with it.
- [x] 2.2 Make the no-flag failure name `--env` and list the registered
      environments, so the fix is visible from the error. Verify a test over a
      registry holding two environments that both names appear.
- [x] 2.3 Verify a mutating command acts on nothing without the flag: a test
      that `remote stop` with a registered `default` environment stops nothing
      and fails naming the flag (the scenario the change exists for).
- [x] 2.4 Verify `--env default` still works exactly as any other name
      (design D2), and that `deploy` — which already required the flag — is
      unchanged.

## 3. Consumers, docs and verification

- [x] 3.1 Update the `internal/remote` tests: delete the ones whose subject is
      the fallback or the legacy file, and move the ones that wrote either as
      a fixture to the registry path, keeping what they assert.
- [x] 3.2 Update `cmd/spinloop` tests that invoke a remote subcommand bare, and
      any example or script that does. Verify nothing invokes one without an
      environment.
- [x] 3.3 Rewrite `docs/commands/remote.md`, where the default environment is
      no longer a fallback, and `docs/env-vars.md`, where the no-`remote.json`
      workflow gains the `--env` it always needed and the identifier it never
      had. Neither mentions the superseded file as read.
- [x] 3.4 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
