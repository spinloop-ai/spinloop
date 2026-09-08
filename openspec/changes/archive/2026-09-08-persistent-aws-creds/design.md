## Context

Every `spinloop remote` call signs with the caller's ambient AWS credentials: Lambda Function URL calls through `sign()` (`internal/remote/remote.go:608`) and direct SDK calls (STS, CloudFormation, EC2, CloudWatch Logs, Pricing) through `LoadAWSConfig` (`internal/remote/aws.go:23`). Both funnel into `awsconfig.LoadDefaultConfig`, so there are exactly two choke points where a stored credential can be introduced. The control-plane stack (`remote/lib/llm-stack.ts`) currently creates only machine principals — `InstanceRole`, `SeedRole`, and the Lambdas' execution roles — and no human-facing principal. Deployment identity is (account, region): the stack name is fixed (`cloud-vm-llm`) and environments live within it. See proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**

- A long-lived AWS credential that survives between SSO log-ins, stored only in the OS keystore (or an owner-only file where no keystore exists), created and removed by `spinloop remote auth`.
- The stored key covers the day-to-day control calls — the `remote` subcommands and the fleet's operations on remote environments; `bootstrap` and `bake` keep requiring ambient administrator credentials.
- `--store` doubles as rotation and can run with the stored key alone, so no admin login is needed after the first store.
- Zero behaviour change for anyone who never stores a key: the standard chain stays the fallback.

**Non-Goals:**

- No SSO device flow or AWS login inside spinloop — the first store still uses whatever credentials the user already has configured.
- No per-environment credential scoping: one entry per region, covering all environments in the account.
- No automatic expiry: access keys do not expire; the ~90-day lifetime is a documented rotation expectation.
- No change to the Lambda authorisation model (Function URLs stay `AWS_IAM`, any principal in the account with the grant can invoke).

## Decisions

### 1. Principal: an IAM user with a stack-owned managed policy, not a role

AWS access keys attach to IAM users only; a role yields at most 12-hour assumed sessions, which would not meet the ~90-day goal. The CDK stack therefore creates an IAM user, `cloud-vm-llm-remote-cli` (fixed name, not deployment-specific, so the public-repo identifier check is unaffected), with a managed policy attached at synth. The policy is a stack-owned `AWS::IAM::ManagedPolicy` resource rather than an inline one because IAM caps the aggregate size of a user's inline policies at 2,048 bytes and the seven control functions plus their log grants do not fit (a managed policy allows 6,144); being a stack resource, it still deletes with the stack, so a re-deploy of an older template version removes it:

- `lambda:InvokeFunctionUrl`, with the `FunctionUrlAuthType=AWS_IAM` condition, on the seven control functions' ARNs (referenced through each URL resource's `FunctionArn` attribute — the URL's own ARN carries a generated id CDK does not expose) — one statement, the same grant `grantInvokeUrl` renders per function.
- The companion `lambda:InvokeFunction` grant with the `InvokedViaFunctionUrl` condition, on the same seven ARNs, one statement — also part of `grantInvokeUrl`'s canonical form.
- `logs:DescribeLogStreams`, `logs:FilterLogEvents`, `logs:GetLogEvents` on the runner and boot log-group ARNs and their streams (`remote logs`).
- `cloudformation:DescribeStacks` on the stack ARN (`Ref: AWS::StackId` — control-plane discovery by `deploy` and `remote auth`).
- `pricing:GetProducts` (resource `*` — the Price List API has no resource-level scoping) so `status --cost` works with the stored key.
- `iam:GetUser`, `iam:ListAccessKeys`, `iam:CreateAccessKey`, `iam:DeleteAccessKey` on the user's own ARN, built from the `AWS::Partition`/`AWS::AccountId` pseudo parameters — self-service rotation. The ARN is not the user's own attribute: the policy attaches to the user, so referencing the user would be a dependency cycle.

Alternatives considered: an inline policy (the 2,048-byte cap — the first attempt with CDK's per-function grant pairs rendered 3.5 KB and failed the deploy with `ServiceLimitExceeded`); having the user assume a role per call (defeats the ~90-day goal).

The user name is a constant on the Go side, not a stack output: `remote auth --store` confirms the user exists with `iam:GetUser` before doing anything, which is also the failure path for control planes deployed before this change.

### 2. Keystore backend: `zalando/go-keyring`, with a file fallback

The OS keystore is accessed through the `zalando/go-keyring` library: it shells out to the `security` CLI on macOS, uses Credential Manager on Windows, and the Secret Service over D-Bus on Linux (pure Go, via godbus). The more widely used `99designs/keyring` was rejected because its macOS backend requires cgo while the release build pins CGO_ENABLED=0 — the shipped binary would silently lack the Keychain backend and every machine would fall back to the file store. `zalando` compiles statically; a `CGO_ENABLED=0` darwin cross-build is part of the verification.

Where the platform keystore is unavailable (headless Linux without a D-Bus Secret Service), or where `SPINLOOP_REMOTE_KEYSTORE=file` selects the file store — a headless macOS session whose keychain is locked or unreachable, and the test suite — the entry is stored in an owner-only file `<config-dir>/keystore/remote-<region>.json` (0600 in a 0700 directory, the same treatment the repo already gives files that may hold secrets). The entry records: access key id, secret, account, user name, region, stored-at timestamp, and which store holds it. `--store` and the no-flag report say which store was used.

The entry is keyed by region: the keyring service is `spinloop-remote` and the region is the entry's user name. The OS keystores offer no way to list a service's entries, so the store keeps a non-secret index entry (the stored regions, one per line) to back the no-flag report; a region the index names but the store no longer holds (removed outside of spinloop) is skipped rather than reported. Keying by (account, region) would be more precise but is a chicken-and-egg: resolving the account requires credentials, which is exactly what the lookup is for. One control plane per account per region is the norm; if two accounts are bootstrapped in the same region, the last stored entry wins and the account recorded in the entry makes the mismatch visible in the report.

Alternatives: `99designs/keyring` (cgo macOS backend conflicts with the static release build); hand-rolled Security.framework/wincred wrappers (cross-platform burden for nothing).

### 3. Precedence at the two choke points

`LoadAWSConfig` becomes the single decision point:

1. If the process environment carries explicit AWS credentials (`AWS_ACCESS_KEY_ID` set) or an explicit profile selection (`AWS_PROFILE` set), load the default chain exactly as today — these win over a stored key. This preserves the existing behaviour where a Spinloop's `.env`/`ENV` injects credentials into the process environment before AWS work (`applySpinloopEnv`), and it lets an operator override the stored key for debugging. The chain-plumbing variables (`AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE`) do not count as explicit: they name where a source lives, not which credential is selected, and counting them would disable the stored key for anyone who has merely relocated their config files.
2. Otherwise, if a stored entry exists for the region, load the default config with an explicit static credentials provider for it (`awsconfig.WithCredentialsProvider`). This skips the rest of the chain, so the stored key beats shared config, SSO sessions, and IMDS.
3. Otherwise, today's behaviour: plain default chain.

`sign()` already calls `LoadAWSConfig`, so Function URL signing, log reading, discovery, and bake polling all inherit this with no per-caller changes. The fleet's remote-node operations — the dashboard and `fleet start`/`stop`/`status`/`keep` on remote environments — sign through the same `sign()` and `LoadAWSConfig`, so they inherit the stored key too; a fleet unit test pins that with a status call where the stored entry is the only resolvable credential. The pricing call (`status --cost`) resolves its credential the same way but keeps the us-east-1 endpoint, since the Price List API is global. `bootstrap` and `bake` are the exception: they call an ambient-only variant (the current `LoadAWSConfig` body), never consulting the keystore.

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
- [Keyring access can prompt for the OS password on some platforms (e.g. a locked macOS Keychain item, Linux Secret Service unlock)] → the prompt comes from the OS, not spinloop; the file fallback avoids keystore access entirely where no keystore exists, `SPINLOOP_REMOTE_KEYSTORE=file` chooses it where a keystore is reachable but unusable, and the report says which store is in use.
- [Two keys per user cap with multiple machines] → the cap is checked before creating; the error names the fix (`--clear` on the other machine, or delete via the console). Rotation on one machine deletes only the key that machine's entry held.
- [Same region, two accounts] → last stored entry wins; the recorded account in the no-flag report makes this visible.
- [`zalando/go-keyring` is a new dependency] → it is the only new Go module, used in one place (`internal/remote`); its Linux path needs a D-Bus session bus only when actually used.
- [CDK rollback: an older template re-deployed deletes the user and its keys] → expected behaviour; stored entries then 403 and the hint says to re-run bootstrap and `--store`.

## Migration Plan

1. Ship the CDK change and the CLI change together (one release). The CLI is backward-compatible: with no stored key, behaviour is unchanged.
2. Users re-run `spinloop remote bootstrap` (idempotent) to add the user, then `spinloop remote auth --store` with their existing admin credentials.
3. No data migration: the registry and `remote.json` formats are untouched.
4. Rollback: redeploying an older CDK template removes the user and its keys; reverting the CLI restores ambient-only resolution. Stored file-fallback entries are inert in either case.

## Open Questions

None — the remaining unknowns (exact keyring library quirks per platform, the `--region` flag's default when nothing else resolves) are implementation details that do not change the specs, the approach, or the task breakdown.
