## Why

A remote endpoint's API key is always minted by the control plane — at deploy,
`ensureEnvApiKey` creates `cloud-vm-llm/<env>/api-key` in Secrets Manager with a
random value, and there is no way to say what the key should be. Every remote
environment therefore gets its own key, so a user cannot share one key across
several remotes or reuse a key they already manage elsewhere. Once remote
environments are first-class fleet nodes, a `fleet.yaml` that lists several
remotes is the natural place to say "these endpoints all take this key" — and
today that is not expressible: an agent pointed at several endpoints needs a
different key per endpoint, and there is no single shared key to give it.

## What Changes

- A fleet-wide API key reference in `fleet.yaml`: a top-level `apiKeyEnv` field
  naming the environment variable that holds the key shared by the fleet's
  remote nodes. Same discipline as every other secret reference in the file —
  the variable's *name*, never the value; resolved from the process environment
  first, then the `.env` beside the file; named-but-unset is a configuration
  error naming the variable. It is the default key for a remote node; a node's
  own `engineTokenEnv` still overrides it. Daemon nodes are unaffected — they
  keep requiring their own `engineTokenEnv` to be gated.
- `spinloop remote deploy` accepts a caller-supplied key as a variable reference
  (`--api-key-env <VAR>`) — never a literal, so the key does not appear on the
  command line. The CLI resolves the variable and sends the value to the deploy
  Lambda in the SigV4-signed request body as a **request-scoped** field: it is
  not part of the persisted deploy-config and never appears in any reply.
- The deploy Lambda stores a supplied key in the environment's existing
  Secrets Manager secret — created if absent, set when supplied — instead of
  always generating one. Everything downstream is unchanged: the instance reads
  the same secret at boot, the env/start Lambdas still report it, and
  `spinloop harness` injects it at launch exactly as today.
- Rotation semantics: deploying *with* a key replaces the environment's secret,
  instantly invalidating the old one; deploying *without* a key leaves the
  existing secret alone (never regenerated). A deploy that replaces the key says
  so in its report (without printing the value), because an agent holding the
  retired key otherwise gets silent 401s.
- **Non-goal:** wiring a fleet-driven deploy that reads the fleet file's
  `apiKeyEnv` automatically. The two sides stay independent and both name
  environment variables, so an operator keeps them in sync by naming the same
  variable in `fleet.yaml` and on the `--api-key-env` flag. A fleet-driven
  deploy can adopt the fleet reference directly later.

## Capabilities

### New Capabilities

(none — this change modifies existing capabilities only)

### Modified Capabilities

- `fleet-config`: a fleet file MAY declare a fleet-wide `apiKeyEnv` naming the
  variable holding the key shared by its remote nodes; it is resolved the way
  every other reference in the file is, and a remote node's own `engineTokenEnv`
  overrides it.
- `fleet-routing`: the "engine key the client sets" requirement — when routing
  selects a *remote* node, the key the launched agent is given is the node's own
  variable when named, otherwise the fleet-wide key; a remote's engine is always
  gated, so a remote that names no key of its own and whose fleet names none
  fails before the agent launches.
- `environment-deployment`: deploy SHALL accept an externally provided API key
  (a variable reference), carry it in the signed request as a request-scoped
  field, and the control plane SHALL store it in the environment's secret —
  create if absent, set when supplied — with the rotation semantics above.
- `remote-endpoint`: the `deploy` subcommand SHALL accept a `--api-key-env`
  flag naming the variable that holds the key to deploy, and the deploy report
  SHALL say a supplied key was applied without printing the value.

## Impact

- Go client: `internal/fleet` (new `Config.APIKeyEnv` field + resolution, and
  the remote-node key path in `select.go`), `internal/remote` (deploy request
  carries a request-scoped key field), `cmd/spinloop` (`deploy`'s `--api-key-env`
  flag and its report line, and the fleet-routed launch's key for a remote node).
- Control plane (`remote/`, TypeScript): the deploy Lambda accepts the
  request-scoped key and `remote/lambda/shared/environments.ts` gains a
  create-or-set for the environment's API-key secret (a `PutSecretValue` for the
  update case).
- Security: the key travels only in the SigV4-signed deploy body and in Secrets
  Manager — never in `remote.json`, never in `fleet.yaml`, never in a reply,
  never persisted in the deploy-config parameter.
- Tests: Go suite (fleet config resolution, remote-node key selection, deploy
  request body, the deploy flag) and `remote/` vitest (the Lambda's
  create-or-set and rotation behaviour). Coverage stays ≥ 80%.
