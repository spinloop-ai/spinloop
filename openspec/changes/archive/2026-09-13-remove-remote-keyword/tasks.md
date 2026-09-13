## 1. Spinloop grammar: remove the REMOTE keyword

- [x] 1.1 Remove `REMOTE` from the parser in `internal/spinloop/spinloop.go`: drop `kwRemote` from the keyword case list (line ~104), the `case kwRemote` assignment (~176), the `REMOTE`+`FLEET` exclusivity check (~189-197), `Selection.Remote` (~55), the `Format` line (~244), and the header comment line (~17); verify `go build ./...` and the suite still compile
- [x] 1.2 Add the dedicated rejection: in `Parse`, when a line's keyword is `remote` (case-insensitive), fail citing that line, saying the `REMOTE` instruction was removed, and naming the replacement (`remote deploy --env <name>` at deploy time, `--env <name>` at apply/unapply/harness); verify with a test asserting the message names the line and `--env` and is not the generic unknown-keyword text (spec: `spinloop-files` "REMOTE is a removed keyword")
- [x] 1.3 Drop `REMOTE` from the generic unknown-keyword error's keyword list (~line 132) and update `internal/spinloop/spinloop_test.go`: the `REMOTE`/`FLEET` tests (~lines 168-222) become rejection tests, `Format` no longer emits a line, `Selection` has no `Remote` field; verify `go test ./internal/spinloop/` passes

## 2. Environment resolution: name-only registry lookup

- [x] 2.1 Rework `remote.IsEnvName` in `internal/remote/environments.go` (~line 56): keep the plain-identifier validation (no path separator), drop its bare-name-vs-path disambiguation framing, and apply it to `--env` values so `--env ./x.json` fails saying an environment name is a plain identifier; verify with a unit test (spec: `remote-environments` "Environment name validity")
- [x] 2.2 Remove the URL form of a remote config: delete `LoadConfigBytes` in `internal/remote/remote.go` (~line 126) with its comment, and any fetch helper it fed; verify `go build ./...` and `go test ./internal/remote/` pass

## 3. The remote subcommands select by --env

- [x] 3.1 Add an `--env <name>` flag to the `remote` subcommands in `cmd/spinloop/remote.go` (status, pause, restart, stop, metrics, env, keep, start, logs, and any others that take config) and replace `resolveRemoteConfig`'s (~line 101) `REMOTE`-driven selection with: flag value → `remote.IsEnvName` validation → registry lookup (`remote.EnvConfigPath` + `LoadConfigFile`), failing "environment %q is not registered: run `spinloop remote deploy --env %q` to create it" when absent; no flag → `remote.LoadDefault` as today; verify with tests in `remote_test.go` / `remote_env_test.go` / `remote_logs_test.go` / `last_active_test.go` for flag selection, default fallback, and the unregistered error (spec: `remote-endpoint` "Environment selection for remote commands")
- [x] 3.2 Keep the Spinloop argument's `ENV`-loading role on those subcommands (`applySpinloopEnv`, remote.go ~line 70) and drop its selection role; an explicit Spinloop argument without a `REMOTE` line is no longer an error; verify with the existing alias/ENV tests in `alias_test.go` and `viper_test.go` updated to the flag (spec: `remote-endpoint` "An explicit Spinloop does not select an environment")
- [x] 3.3 Delete the now-dead helpers in `cmd/spinloop/remote.go`: `resolveRemotePath` (~137), `remoteConfig` (~179), `remoteBaseURL` (~213), `remoteEnvName` (~226), `resolveRemoteConfigForSpinloop` (~245); verify `go build ./...` and no remaining references
- [x] 3.4 Rename `remote start`'s `-e`/`--env` bool to `--print-env` with no short form (flag ~line 418, help text ~line 405); verify `remote start --print-env` prints the export lines and `-e` is no longer accepted (test in `remote_test.go`; spec: `remote-env` "start --print-env flag prints exports")
- [x] 3.5 Make `remote deploy` require `--env <name>`: the flag is mandatory (`deriveDeployTarget`, remote.go ~line 1501, stops reading the name from the Spinloop — the `env = sel.Remote` at ~line 1520 goes); a deploy without the flag fails naming the missing flag; verify with `remote_deploy_test.go` (spec: `environment-deployment` "Deploy names its environment with a required flag")

## 4. apply / unapply / harness take --env

- [x] 4.1 Add `--env/-e <name>` to `apply` (`applyCmd`, main.go ~line 451) and `unapply` (`unapplyCmd`, ~line 497), and to the harness launch command where `--fleet/-f` is defined (`commands.go` ~line 269)
- [x] 4.2 Rework `applySelection` (main.go ~191): the flag's name keys the provider, sets the `<env>/<model>` default model and the display label, and — when the Spinloop states no `BASEURL` — takes the base URL from the named environment's registered `remote.json` (reporting that it did); a `BASEURL` in the Spinloop still wins for the address; an unregistered name fails "environment %q is not registered: run `spinloop remote deploy --env %q` to create it"; the `sel.Remote` reads at ~211 and ~231 go; verify with `apply_test.go` (spec: `remote-environments` "An environment names the harness provider" + "Remote provider display name", `provider-selection` "Base URL resolution")
- [x] 4.3 Rework `removeSelection` (main.go ~878) to resolve the provider name from the same flag instead of `sel.Remote` (~879); verify with `apply_test.go` unapply tests (spec: `remote-environments` "An environment names the harness provider", unapply scenario)
- [x] 4.4 Rework `fetchRemoteEnv` (main.go ~1233) to take the environment name from the flag and fetch via the registry's `remote.json` (the `resolveRemoteConfigForSpinloop` call at ~1239 goes); verify with `harness_remote_test.go` (spec: `harness-remote-env` "harness injects the endpoint's credentials")
- [x] 4.5 Wire the flag through `applyBeforeLaunch` (main.go ~1156) and the launch's `routeOptions` (route.go ~line 24): a launch given `--env` alongside a fleet file — the `--fleet` flag or the `./fleet.yaml` in force when the Spinloop is not named — fails naming both; verified with `harness_remote_test.go` (spec: `harness-remote-env` "--env and a fleet conflict" scenario, `fleet-routing` delta)
- [x] 4.6 Pass the node name as the environment in `fleet deploy`: `deployOneNode` (fleet.go ~line 667) hands the node name to the deploy path instead of `deriveDeployTarget` reading a `REMOTE` line, with no override flag; verify with `fleet_deploy_test.go` (spec: `environment-deployment` "Fleet deploy uses the node name")

## 5. Tests across the CLI

- [x] 5.1 Update the remaining REMOTE-era tests so the whole suite reflects the flag: `cmd/spinloop/{alias_test.go, apply_test.go, fleet_deploy_test.go, harness_remote_test.go, last_active_test.go, remote_auth_test.go, remote_deploy_test.go, remote_env_test.go, remote_environments_test.go, remote_logs_test.go, remote_seed_test.go, remote_test.go, route_test.go, url_source_test.go, viper_test.go}` and `internal/remote/{remote_test.go, seed_test.go}`; verify `go test ./...` passes
- [x] 5.2 Confirm coverage stays at or above 80% total; verify `go test ./... -cover` reports it

## 6. Docs and examples

- [x] 6.1 Remove the `REMOTE` line from `examples/fleet-remote/qwen/Spinloop` (line 8) and `examples/fleet-remote/llama/Spinloop` (line 8); verify `go build ./...` and any example-driven test that parses them still passes
- [x] 6.2 Update `docs/spinloop-file.md`: drop `REMOTE` from the keyword table and the examples (~lines 68-114, 142-144, 169) and document that a `REMOTE` line now fails with the migration error; verify the file no longer mentions a working `REMOTE` instruction
- [x] 6.3 Update `docs/commands/remote.md`: the config-discovery section (~lines 117-134) becomes the `--env` flag with the `default` fallback, deploy's required `--env` (~lines 349-386), `start`'s `--print-env` rename, and the link list (~line 448); verify the doc describes no path/URL form
- [x] 6.4 Update `docs/commands/apply.md` (~line 36) and `docs/commands/fleet.md` (the `REMOTE_ENGINE_KEY` reference at ~line 214 stays — it is a key-env var name, not the keyword; confirm no other `REMOTE`-instruction references), and `docs/env-vars.md` for the `--env` flow; verify `grep -rn "REMOTE" docs/` shows only `SPINLOOP_REMOTE_*` env vars and the `REMOTE_ENGINE_KEY` variable

## 7. Final verification

- [x] 7.1 Run `gofmt -l .` (expect no output), `go vet ./...`, `go build -o spinloop ./cmd/spinloop`, and `go test ./... -cover`; verify all pass with coverage >= 80%
- [x] 7.2 Smoke-test the built binary: `./spinloop apply Spinloop` on a file with a `REMOTE` line fails with the migration error naming `--env`; `./spinloop remote status --help` shows `--env` and `./spinloop remote start --help` shows `--print-env` and no `-e`; verify the observed output matches the spec scenarios
