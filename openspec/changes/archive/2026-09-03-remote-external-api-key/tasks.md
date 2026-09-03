# Tasks

## 1. Fleet: fleet-wide key reference (internal/fleet)

- [x] 1.1 Add `APIKeyEnv` (yaml `apiKeyEnv`) to `Config` in `internal/fleet/config.go`, beside `NodeConfig.EngineTokenEnv`
- [x] 1.2 Add the fleet-level resolver (process environment first, then the `.env` beside the fleet file, reusing the `resolveTokenEnv` lookup); a named-but-unset variable is an error naming the variable and the fleet file
- [x] 1.3 Unit tests: unset fleet variable is fine until resolved, set variable resolves, named-but-unset fails naming the variable and file, `.env`-beside-the-file fills a gap
- [x] 1.4 `yaml.Unmarshal` ignores an unknown `apiKeyEnv` on an old binary — confirm with a parse test that a fleet file with `apiKeyEnv` loads without error

## 2. Fleet: remote key resolution (internal/fleet/select.go)

- [x] 2.1 In `engineKeyFor`, special-case `n.Kind == KindRemote`: the key is the node's own `engineTokenEnv` when named, else the fleet's `apiKeyEnv`; daemon logic unchanged
- [x] 2.2 A remote with no resolvable key (neither named, or the variable set nowhere) fails before launch, naming the node and what to set
- [x] 2.3 Unit tests: fleet key used for a remote, node `engineTokenEnv` overrides the fleet key, no key fails early with the node named, unset variable fails naming the variable, a daemon node is unaffected by the fleet key
- [x] 2.4 Confirm the wake path is unchanged: a remote wake candidate is still refused by `StartWith`, and a daemon wake still resolves only the node's own `engineTokenEnv`

## 3. Remote deploy: externally provided key (Go)

- [x] 3.1 `internal/remote/remote.go`: add `APIKey string \`json:"apiKey,omitempty"\`` to `Deploy`'s request-scoped body struct beside `allowedCidr`/`reseed`, and a `apiKey` parameter to `Deploy`
- [x] 3.2 Test: the request body carries `apiKey` only when a key is supplied, and is byte-identical to today's body when it is not; the key never enters `DeployConfig` marshalling
- [x] 3.3 `cmd/spinloop/remote.go`: add the `--api-key-env <VAR>` flag to `deploy`, registered on its own flag set; resolve it from the process environment (after `applySpinloopEnv`), failing naming the variable before anything is sent
- [x] 3.4 Deploy report: print the key action from the control-plane reply (`api key: created` / `api key: rotated`), never the value
- [x] 3.5 Tests through the existing `httptest` seam: a supplied key reaches the request body, an unset variable fails before the request is made, the report names the action without the value

## 4. Control plane: store or rotate the key (remote/lambda)

- [x] 4.1 `remote/lambda/shared/environments.ts`: `ensureEnvApiKey(env, providedKey?)` — supplied key creates the secret if absent and sets its value (`PutSecretValueCommand`, adding the import) if present; no key keeps today's generate-only-if-absent and never regenerates; return the action taken (`created` / `rotated` / none)
- [x] 4.2 `remote/lambda/deploy/index.ts`: read the request-scoped `apiKey` from the raw body the way `allowedCidr`/`reseed` are read (not from `parseDeployConfig`), pass it to `ensureEnvApiKey`, and include the action word in the reply — never the value; the `console.log` summary does not gain the key
- [x] 4.3 vitest: key + no secret creates it with the supplied value, key + existing secret calls `PutSecretValue` and reports rotated, no key leaves an existing secret untouched (no `PutSecretValue` call) and reports no action, the reply body never contains the value
- [x] 4.4 Confirm the persisted deploy-config (SSM parameter) is written from `parseDeployConfig` only, so a supplied key is not persisted — add an assertion if one is not already present

## 5. Documentation

- [x] 5.1 `remote/docs/architecture.md`: document that a deploy with a key replaces the environment's API key (rotation; the old key stops working) and that omitting it leaves the existing key in place
- [x] 5.2 Update the user-facing docs (`docs/`) and `AGENTS.md` where the fleet file fields and the `remote deploy` flags are described, adding `apiKeyEnv` (remote-only default) and `--api-key-env <VAR>`

## 6. Verification

- [x] 6.1 `gofmt -l .` clean, `go vet ./...` clean
- [x] 6.2 `go test ./... -cover` green with total coverage still >= 80%
- [x] 6.3 `pnpm test` in `remote/` green, including the deploy tests
- [x] 6.4 `scripts/check-no-cloud-identifiers.sh` clean
