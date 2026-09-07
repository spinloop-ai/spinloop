# Persistent AWS Credentials

## Why

Every `spinloop remote` call is signed with the caller's ambient AWS credentials — an SSO session, a profile, or environment variables. SSO sessions expire, so operating a remote endpoint means logging back into AWS on a regular cycle (issue #172: "avoid regular log-ins and token expiration"). Spinloop stores no credentials of its own today, so there is no way to keep a remote endpoint reachable between log-ins.

## What Changes

- A new `spinloop remote auth` subcommand:
  - `--store` creates an AWS access key for a control-plane IAM user (created by the CDK stack) and stores it in the OS keystore — Keychain on macOS, Credential Manager on Windows, Secret Service on Linux — keyed by region. When a key is already stored, `--store` rotates: the stored key creates the replacement, the new key is verified against the same account, the entry is swapped, and the old key is deleted — no administrator credentials needed.
  - `--clear` removes the stored entry and deletes the access key on the AWS side (best effort, so a cleared key does not linger).
  - With no flag it reports what is stored — account, region, user, access key id, when stored — never the secret.
- The CDK control-plane stack (`remote/lib/llm-stack.ts`) gains an IAM user (`cloud-vm-llm-remote-cli`) carrying a narrow policy: invoke the seven control-plane function URLs, read the `/cloud-vm-llm/*` CloudWatch log groups, `cloudformation:DescribeStacks` on the stack, and manage its own access keys. Access keys attach to IAM users, not roles, so this is the principal a long-lived keypair can belong to.
- Credential resolution for `spinloop remote` (and fleet's remote nodes, which share the same client) consults the OS keystore for the target region. Explicit AWS environment credentials or a named profile override the stored key; otherwise the stored key takes precedence over the rest of the standard chain (shared config, SSO sessions, IMDS), which remains the fallback.
- `bootstrap` and `bake` keep using ambient administrator credentials and explicitly ignore any stored key: they provision the control plane itself, and the stored key's policy does not cover them.
- A control plane deployed before this change has no such user; `remote auth --store` then says to re-run `spinloop remote bootstrap` first.
- Access keys do not expire. The ~90-day lifetime is a rotation expectation: `--store` is the rotation, and it can be run with the stored key alone.

## Capabilities

### New Capabilities

- `remote-auth`: the `spinloop remote auth` command and the stored long-lived control-plane credential — keystore storage, lookup precedence, self-rotation, and clearing.

### Modified Capabilities

- `remote-endpoint`: the "Authenticated control requests" requirement — resolution from the standard chain and "Spinloop SHALL NOT store AWS credentials of its own" — changes so that an OS-keystore-stored key is a credential source, overridable by explicit environment credentials or a profile; the "Remote command group" requirement gains the `auth` subcommand.
- `endpoint-provisioning`: the stack bootstrap deploys gains the control-plane IAM user and its policy, and the bootstrap plan and report name it.

## Impact

- Go: `internal/remote` gains a keystore-backed credential source consulted at the two choke points (`LoadAWSConfig`, `sign`); new `cmd/spinloop/remote_auth.go`; `bootstrap` and `bake` explicitly bypass the stored key. New Go dependency: an OS keyring library (e.g. `99designs/keyring`), with an owner-only file under spinloop's config directory as the fallback where no keystore exists (headless Linux).
- CDK: `remote/lib/llm-stack.ts` adds the IAM user, its policy, and a stack output naming the user; existing deployments need a re-bootstrap before `remote auth --store` works.
- Docs: `docs/commands/remote.md`, `README.md`, `remote/README.md` — the credential story moves from "ambient only" to "stored key, ambient as fallback".
- Public-repo constraint unchanged: the user name is fixed, not deployment-specific, so `scripts/check-no-cloud-identifiers.sh` has nothing new to catch.
