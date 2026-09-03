# Design

## Context

Two independent seams already carry the pieces this change joins:

- `fleet.yaml` names secrets by **reference only** — a node's `tokenEnv` and
  `engineTokenEnv` are variable *names*, resolved from the process environment
  first, then the `.env` beside the file (`internal/fleet/config.go`,
  `resolveTokenEnv`). A reference that resolves to nothing is a configuration
  error naming the variable.
- The deploy flow already carries **request-scoped** fields beside the
  persisted config: `allowedCidr` and `reseed` ride the SigV4-signed body, are
  read from the raw body in the Lambda, and are deliberately **not** part of the
  `DeployConfig` that is stored verbatim to SSM and read back on every wake
  (`internal/remote/remote.go` `Deploy`, `remote/lambda/deploy/index.ts`).
- The control plane mints each environment's key with `ensureEnvApiKey`, which
  generates a random value on first creation and otherwise leaves the secret
  alone (`remote/lambda/shared/environments.ts`).
- A fleet-routed launch already injects the chosen node's key into the agent
  from `Choice.APIKey` (`cmd/spinloop/commands.go`, `main.go`), so once routing
  resolves a remote's key the agent picks it up with no new injection code.

See proposal.md for the motivation.

## Goals / Non-Goals

**Goals**

- A fleet can name one key shared by its remote nodes, resolved with the same
  reference discipline as every other secret in `fleet.yaml`.
- `spinloop remote deploy` can store a caller-chosen key in an environment's
  secret, with explicit, visible rotation.
- The key travels only in the signed deploy body and in Secrets Manager —
  never in `fleet.yaml`, `remote.json`, the persisted deploy-config, a reply, or
  a log line.

**Non-Goals**

- No fleet-driven deploy that reads `fleet.yaml`'s `apiKeyEnv` automatically
  (deploy does not read the fleet file today; that is a later, separate change
  that can adopt the reference directly).
- No change to how a `REMOTE` Spinloop fetches its key at launch (the env Lambda
  path is untouched — it now simply returns the shared key).
- No key material in any persistent spinloop-owned file.

## Decisions

### 1. Fleet field is `apiKeyEnv`, a remote-only default

`Config` gains `APIKeyEnv string` (yaml `apiKeyEnv`), resolved by a small
helper that reuses the existing env-then-`.env` lookup but attributes an error
to the fleet file rather than a node. It is the **default key for remote
nodes only**: a remote's effective key is its own `engineTokenEnv` when named,
else the fleet's `apiKeyEnv`. Daemon nodes are unchanged — they still gate only
on their own `engineTokenEnv`.

*Why remote-only:* the field's purpose is to share a key across remotes. Making
it a default for *all* nodes would let a fleet that adds `apiKeyEnv` (to share
a remote key) suddenly gate its daemon nodes with a key their engines were never
set up to accept, breaking them. Scoping it to remotes keeps the change additive
and opt-in.

*Alternative considered:* a fleet-wide default for every node. Rejected for the
reason above.

### 2. Deploy takes a variable reference, not a literal

`spinloop remote deploy` gains `--api-key-env <VAR>`. The CLI resolves `$VAR`
from the **process environment** — which `applySpinloopEnv` has already populated
with the Spinloop's `.env` and `ENV` lines — and sends the value. A named variable
set nowhere fails the deploy before anything is sent.

*Why a reference:* a literal on the command line is visible in `ps` and shell
history. The codebase already refuses literal engine keys (the daemon's
`--api-token-file`, the engine's `--api-key-file`); a variable reference is the
consistent choice.

*Alternatives considered:* `--api-key <literal>` (rejected — `ps` exposure);
`--api-key-file <path>` (works, but the value crosses the signed network
anyway, so a file is more machinery than the reference needs, and the reference
reuses the `applySpinloopEnv` population for free).

### 3. The key rides request-scoped, beside `allowedCidr` and `reseed`

`Deploy`'s body struct gains `APIKey string \`json:"apiKey,omitempty"\``
alongside the existing request-scoped fields; `Deploy` takes the resolved value
as a parameter. It is **not** added to `DeployConfig`, so it is never written to
the SSM deploy-config and never read back on a wake. `omitempty` keeps the
request byte-identical when no key is supplied, so an older control plane that
predates the field simply ignores it.

*Why:* `DeployConfig` is persisted verbatim and re-sent on every wake. A key in
it would be stored in SSM and echoed by any read of the config. `allowedCidr`
and `reseed` solve the identical problem the identical way; the key joins them.

### 4. The Lambda create-or-sets the secret, with explicit rotation

`ensureEnvApiKey(env)` becomes `ensureEnvApiKey(env, providedKey)` and reports
what it did:

- `providedKey` set, secret absent → `CreateSecretCommand` with that value
  ("created").
- `providedKey` set, secret present → `PutSecretValueCommand` with that value
  ("rotated"). The old key is invalid immediately.
- `providedKey` empty → existing behaviour: create-and-generate if absent,
  otherwise leave the secret **alone** ("unchanged"). It is never regenerated.

The Lambda adds the action word (`apiKey: "created" | "rotated"`) to its reply —
the action, never the value — and the deploy report prints it
(`  api key: rotated`), so a rotation out from under a live agent is visible
rather than surfacing later as 401s.

*Why leave-alone-when-absent:* regenerating on a no-key redeploy would silently
invalidate a key the operator still holds. Leaving it alone is the safe default;
rotation is only ever explicit.

### 5. Fleet key and deploy flag stay independent

Both reference environment variables; the operator keeps them in sync by naming
the same variable in `fleet.yaml` (`apiKeyEnv: SHARED_KEY`) and on the deploy
flag (`--api-key-env SHARED_KEY`). Deploy does not read the fleet file, so there
is no automatic wiring.

*Why:* the issue scopes these as "complementary rather than dependent". Adding
a `--fleet` flag to deploy (or auto-detecting `./fleet.yaml` and checking
membership) is new machinery for a benefit that only lands once fleet-driven
deploys exist. Keeping them independent now means that later change can adopt
the reference directly without reworking this one.

### 6. A selected remote is always given a key, or the launch fails

In `select.go`, `engineKeyFor` special-cases `n.Kind == KindRemote`: the key is
the node's own `engineTokenEnv` if named, else the fleet's `apiKeyEnv`; if
neither names a variable — or the variable it names is set nowhere — routing
fails before the agent launches, naming the node and what to set. A remote is
never reached ungated. The value flows into the existing `Choice.APIKey` →
`OPENAI_API_KEY` injection, so no new injection code is needed. Daemon handling
in `engineKeyFor` is unchanged.

*Why fail early:* a remote whose key cannot be resolved would hand the agent an
endpoint that 401s. Failing before launch, naming what to set, matches the
existing "gated node with no key fails early" behaviour for daemons.

## Risks / Trade-offs

- **Rotation invalidates a live agent's key.** → The reply and report carry the
  explicit action word ("rotated"), and `remote/docs/architecture.md` documents
  that a key-supplied deploy replaces the environment's key.
- **The key leaks into a log or reply.** → Neither the Lambda's `console.log`
  nor the Go deploy summary prints the value; the reply carries only the action
  word. Tests assert the supplied key is absent from the reply body and from the
  persisted deploy-config parameter.
- **Existing remotes hold minted keys the operator cannot read.** → Migration:
  `spinloop remote env <env>` prints the current key, or a redeploy with
  `--api-key-env` sets a known one. Documented in the migration plan.
- **New CLI against an old control plane.** → `omitempty` means a supplied key
  is silently ignored (the key is still minted); the deploy succeeds. The gap is
  one-directional and benign; deploying the Lambda closes it.
- **A fleet adds `apiKeyEnv` and a remote it lists now requires a key.** → That
  is the opt-in. Such a remote previously 401'd at request time (the agent got
  no key); it now either works or fails earlier with a clear message. Strictly
  an improvement. An old binary reading a new fleet file is unaffected:
  `yaml.Unmarshal` ignores the unknown `apiKeyEnv` field.

## Migration Plan

1. Ship the control-plane change (Lambda + `ensureEnvApiKey`) and the Go client
   together or the Lambda first — the request field is `omitempty`, so an old
   Lambda ignores it and a new Lambda with no key behaves exactly as today.
2. For a fleet that already lists remotes: to route to them, name the key.
   Either add `apiKeyEnv` (or a per-node `engineTokenEnv`) in `fleet.yaml`
   pointing at a variable the operator holds — obtainable with
   `spinloop remote env <env>` — or redeploy the remotes with
   `--api-key-env <VAR>` to set a known shared key.
3. Rollback: revert the code. The `apiKey` body field and the `apiKeyEnv` fleet
   field are both ignored by the other side, so a mixed version state is safe.
