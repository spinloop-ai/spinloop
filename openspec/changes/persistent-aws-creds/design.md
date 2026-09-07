## Context

Every `spinloop remote` call signs with the caller's ambient AWS credentials: Lambda Function URL calls through `sign()` (`internal/remote/remote.go:608`) and direct SDK calls (STS, CloudFormation, EC2, CloudWatch Logs, Pricing) through `LoadAWSConfig` (`internal/remote/aws.go:23`). Both funnel into `awsconfig.LoadDefaultConfig`, so there are exactly two choke points where a stored credential can be introduced. The control-plane stack (`remote/lib/llm-stack.ts`) currently creates only machine principals — `InstanceRole`, `SeedRole`, and the Lambdas' execution roles — and no human-facing principal. Deployment identity is (account, region): the stack name is fixed (`cloud-vm-llm`) and environments live within it. See proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**

- A long-lived AWS credential that survives between SSO log-ins, stored only in the OS keystore (or an owner-only file where no keystore exists), created and removed by `spinloop remote auth`.
- The stored key covers the day-to-day `remote` commands only; `bootstrap` and `bake` keep requiring ambient administrator credentials.
- `--store` doubles as rotation and can run with the stored key alone, so no admin login is needed after the first store.
- Zero behaviour change for anyone who never stores a key: the standard chain stays the fallback.

**Non-Goals:**

- No SSO device flow or AWS login inside spinloop — the first store still uses whatever credentials the user already has configured.
- No per-environment credential scoping: one entry per region, covering all environments in the account.
- No automatic expiry: access keys do not expire; the ~90-day lifetime is a documented rotation expectation.
- No change to the Lambda authorisation model (Function URLs stay `AWS_IAM`, any principal in the account with the grant can invoke).

## Decisions

### 1. Principal: an IAM user with an inline policy, not a role

AWS access keys attach to IAM users only; a role yields at most 12-hour assumed sessions, which would not meet the ~90-day goal. The CDK stack therefore creates an IAM user, `cloud-vm-llm-remote-cli` (fixed name, not deployment-specific, so the public-repo identifier check is unaffected), with an inline policy attached at synth:

- `lambda:InvokeFunctionUrl` on the seven function-URL ARNs (`startUrl.functionUrlArn` et al.).
- `logs:DescribeLogStreams`, `logs:FilterLogEvents`, `logs:GetLogEvents` on the runner and boot log-group ARNs (`remote logs`).
- `cloudformation:DescribeStacks` on `this.stackArn` (control-plane discovery by `deploy` and `remote auth`).
- `pricing:GetProducts` (resource `*` — the Price List API has no resource-level scoping) so `status --cost` works with the stored key.
- `iam:GetUser`, `iam:ListAccessKeys`, `iam:CreateAccessKey`, `iam:DeleteAccessKey` on the user's own ARN — self-service rotation.

Alternatives considered: a customer-managed policy (a named, reusable policy is nicer in the IAM console, but it outlives stack deletion unless specially handled — an inline policy deletes with the stack, which is what a re-deploy of an older template version should do); having the user assume a role per call (defeats the ~90-day goal).

The user name is a constant on the Go side, not a stack output: `remote auth --store` confirms the user exists with `iam:GetUser` before doing anything, which is also the failure path for control planes deployed before this change.

### 2. Keystore backend: `99designs/keyring`, with a file fallback

The OS keystore is accessed through the `99designs/keyring` library (Keychain on macOS, Credential Manager on Windows, Secret Service on Linux). Where the platform keystore is unavailable (headless Linux without a D-Bus Secret Service), the entry is stored in an owner-only file `<config-dir>/keystore/remote-<region>.json` (0600 in a 0700 directory, the same treatment the repo already gives files that may hold secrets). The entry records: access key id, secret, account, user name, region, stored-at timestamp, and which store holds it. `--store` and the no-flag report say which store was used.

The entry is keyed by region (`spinloop-remote-<region>` in the keyring). Keying by (account, region) would be more precise but is a chicken-and-egg: resolving the account requires credentials, which is exactly what the lookup is for. One control plane per account per region is the norm; if two accounts are bootstrapped in the same region, the last stored entry wins and the account recorded in the entry makes the mismatch visible in the report.

Alternatives: `zalando/go-keyring` (less actively maintained); hand-rolled Security.framework/wincred wrappers (cross-platform burden for nothing).

### 3. Precedence at the two choke points

`LoadAWSConfig` becomes the single decision point:

1. If the process environment carries explicit AWS credentials (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` set) or an explicit profile selection (`AWS_PROFILE`, `AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE`), load the default chain exactly as today — these win over a stored key. This preserves the existing behaviour where a Spinloop's `.env`/`ENV` injects credentials into the process environment before AWS work (`applySpinloopEnv`), and it lets an operator override the stored key for debugging.
2. Otherwise, if a stored entry exists for the region, load the default config with an explicit static credentials provider for it (`awsconfig.WithCredentialsProvider`). This skips the rest of the chain, so the stored key beats shared config, SSO sessions, and IMDS.
3. Otherwise, today's behaviour: plain default chain.

`sign()` already calls `LoadAWSConfig`, so Function URL signing, log reading, discovery, and bake polling all inherit this with no per-caller changes. `bootstrap` and `bake` are the exception: they call an ambient-only variant (the current `LoadAWSConfig` body), never consulting the keystore.

The expired-credentials hint is source-aware: when the stored key was the credential in use, the "refresh" guidance says `spinloop remote auth --store` instead of "refresh your SSO session".

### 4. The `auth` command

`cmd/spinloop/remote_auth.go`, registered under `remote` alongside `bootstrap` and `bake`. It takes no Spinloop path; it takes a `--region` flag resolved with the same precedence as bootstrap's (`resolveRegion`), plus `--store` and `--clear`, which are mutually exclusive.

`--store`:

1. Resolve the region; load ambient credentials (never the keystore).
2. `iam:GetUser` for the control-plane user; absent → "re-run `spinloop remote bootstrap` first".
3. If an entry is already stored for the region: rotation mode — the stored credential creates the replacement key (a separate IAM client configured with the stored static credentials), the entry is swapped, and the superseded key is deleted. Otherwise the ambient credentials create the first key.
4. Respect IAM's two-keys-per-user cap: if the user already has two keys and the store is not a rotation, fail and say so (the other key is likely on another machine — `--clear` there first).
5. Verify the new key with STS `GetCallerIdentity`: it must resolve to the same account as the caller, or it is not stored.
6. Write the entry and confirm: account, region, user, access key id, store used. The secret is never printed.

`--clear`:

1. Resolve the region and look up the entry; none → say so and exit 0 (idempotent).
2. Delete the access key on the AWS side using the stored credential, best effort: on failure the local entry is still removed and the command reports that the key may still exist on the AWS side.
3. Remove the local entry.

No flag: list every stored entry (account, region, user, access key id, stored-at, store) from the local store only — no AWS call, no secret. Nothing stored → say so and name `--store`.

Following the repo's existing test pattern (package-variable seams as in `remote_bootstrap.go`), the keyring operations and the IAM/STS calls are behind seams so the flows are unit-testable without AWS, a network, or a real keystore.

### 5. Existing deployments

A control plane deployed before this change has no such user. Re-running `spinloop remote bootstrap` is idempotent and adds it. Until then, `remote auth --store` fails with that guidance, and every other command behaves exactly as before (no stored key, standard chain).

## Risks / Trade-offs

- [A long-lived unattended key in a keystore is a standing credential] → the policy is scoped to day-to-day control only (no stack create/delete, no bake, no instance creation); `--clear` deletes the AWS-side key; rotation is one command.
- [Keyring access can prompt for the OS password on some platforms (e.g. a locked macOS Keychain item, Linux Secret Service unlock)] → the prompt comes from the OS, not spinloop; the file fallback avoids keystore access entirely where no keystore exists, and the report says which store is in use.
- [Two keys per user cap with multiple machines] → the cap is checked before creating; the error names the fix (`--clear` on the other machine, or delete via the console). Rotation on one machine deletes only the key that machine's entry held.
- [Same region, two accounts] → last stored entry wins; the recorded account in the no-flag report makes this visible.
- [`99designs/keyring` is a new dependency] → it is the only new Go module, used in one place (`internal/remote`); its Linux path needs D-Bus only when actually used.
- [CDK rollback: an older template re-deployed deletes the user and its keys] → expected behaviour; stored entries then 403 and the hint says to re-run bootstrap and `--store`.

## Migration Plan

1. Ship the CDK change and the CLI change together (one release). The CLI is backward-compatible: with no stored key, behaviour is unchanged.
2. Users re-run `spinloop remote bootstrap` (idempotent) to add the user, then `spinloop remote auth --store` with their existing admin credentials.
3. No data migration: the registry and `remote.json` formats are untouched.
4. Rollback: redeploying an older CDK template removes the user and its keys; reverting the CLI restores ambient-only resolution. Stored file-fallback entries are inert in either case.

## Open Questions

None — the remaining unknowns (exact keyring library quirks per platform, the `--region` flag's default when nothing else resolves) are implementation details that do not change the specs, the approach, or the task breakdown.
