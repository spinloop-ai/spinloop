## 1. Keystore entry and storage backend

- [ ] 1.1 Add the stored-credential entry type (access key id, secret, account, user name, region, stored-at, store used) and the keyring operations (put, get, delete, list) in `internal/remote`, keyed by region with a `spinloop-remote-<region>` name, backed by `99designs/keyring` with an owner-only file fallback (`<config-dir>/keystore/remote-<region>.json`, 0600 in a 0700 dir) where no OS keystore exists; verify with unit tests covering put/get/delete/list on a fake keyring, region keying, and the file fallback's modes (go test ./internal/remote)
- [ ] 1.2 Add the `99designs/keyring` dependency to go.mod; verify `go mod tidy` and `go build ./...` succeed

## 2. Credential resolution at the choke points

- [ ] 2.1 Split `LoadAWSConfig` (`internal/remote/aws.go`) into an ambient-only loader (today's body) and a loader that applies the precedence: explicit AWS env credentials or explicit profile selection → default chain as today; else a stored entry for the region → default config with a static credentials provider for it; else default chain; verify with unit tests for the full precedence matrix, including env-over-stored, profile-over-stored, stored-over-SSO/shared-config, and no-stored-falls-back
- [ ] 2.2 Make the expired-credential hint in `sign()` (`internal/remote/remote.go`) source-aware: when the stored key was the credential in use, the refresh guidance says `spinloop remote auth --store`; verify with unit tests on both hint variants
- [ ] 2.3 Point `bootstrap` and `bake` credential preflights (`cmd/spinloop/remote_bootstrap.go`) at the ambient-only loader so they never consult the keystore; verify with a unit test that a stored entry is not used by the bootstrap preflight and that the existing preflight tests still pass

## 3. CDK: control-plane IAM user and policy

- [ ] 3.1 Add the `cloud-vm-llm-remote-cli` IAM user to `remote/lib/llm-stack.ts` with an inline policy granting: `lambda:InvokeFunctionUrl` on the seven function-URL ARNs, `logs:DescribeLogStreams`/`FilterLogEvents`/`GetLogEvents` on the runner and boot log-group ARNs, `cloudformation:DescribeStacks` on the stack ARN, `pricing:GetProducts`, and `iam:GetUser`/`ListAccessKeys`/`CreateAccessKey`/`DeleteAccessKey` on the user's own ARN; verify by extending `remote/test/stack.test.ts` to assert the user, each grant, and the absence of any provisioning permission (no cloudformation:CreateStack, no imagebuilder, no ec2:RunInstances) and running `pnpm test` in `remote/`
- [ ] 3.2 Confirm the fixed user name introduces no deployment identifier; verify `scripts/check-no-cloud-identifiers.sh` passes

## 4. The `remote auth` command

- [ ] 4.1 Register `auth` under the `remote` command group in `cmd/spinloop/remote_auth.go` with mutually exclusive `--store`/`--clear` flags and a `--region` flag resolved like bootstrap's; verify `spinloop remote auth --help` renders per the cli-ux conventions and that `spinloop remote --help` lists `auth`
- [ ] 4.2 Implement `--store` first-store: load ambient credentials only, confirm the control-plane user with `iam:GetUser` (absent → fail naming `spinloop remote bootstrap`), create the access key, verify it with STS `GetCallerIdentity` resolves to the caller's account (mismatch → not stored, fail), write the entry, confirm with account/region/user/key-id/store and never the secret; verify with unit tests through seams for the happy path, the missing-user path, and the account-mismatch path
- [ ] 4.3 Implement `--store` rotation: when an entry exists for the region, create the replacement with the stored credential (separate IAM client, no ambient credential required), swap the entry, and delete the superseded key; check IAM's two-keys-per-user cap when not rotating and fail naming the fix; verify with unit tests including rotation with no ambient credentials configured
- [ ] 4.4 Implement `--clear`: look up the entry (none → say so, exit 0), delete the access key on the AWS side with the stored credential best-effort (on failure still remove locally and report the key may linger), remove the local entry; verify with unit tests for the happy path and the AWS-deletion-failure path
- [ ] 4.5 Implement the no-flag report: list every stored entry (account, region, user, access key id, stored-at, store) from the local store only, no AWS call, no secret; nothing stored → say so and name `--store`; verify with unit tests for both cases
- [ ] 4.6 Add `auth` to tab completion where the `remote` subcommands are completed (`cmd/spinloop/complete.go`); verify the existing completion tests pass and `__complete` never errors

## 5. Docs

- [ ] 5.1 Update `docs/commands/remote.md`: an `auth` section (store, report, clear, rotate) and the credentials section — stored key with ambient as fallback, explicit env/profile override, bootstrap/bake stay admin-only; verify the claims match the implemented behaviour by reading the final code
- [ ] 5.2 Update `README.md` (the credentials paragraph) and `remote/README.md` (prerequisites: bootstrap still admin; day-to-day commands can use the stored key; existing control planes need a re-bootstrap before `--store`; rotate roughly every 90 days); verify by reading that no claim contradicts the implementation or the identifier check

## 6. Verification

- [ ] 6.1 Run the full suite with coverage and the linters; verify `go test ./... -cover` keeps total coverage >= 80%, `go vet ./...` and `gofmt -l .` are clean, and `pnpm test` passes in `remote/`
- [ ] 6.2 End-to-end against a real account: re-run `spinloop remote bootstrap`, run `spinloop remote auth --store`, log out of SSO, and verify `spinloop remote status` succeeds with the stored key, that `spinloop remote auth` reports the entry, that `--store` again rotates without admin credentials, and that `--clear` removes both the entry and the AWS-side key
