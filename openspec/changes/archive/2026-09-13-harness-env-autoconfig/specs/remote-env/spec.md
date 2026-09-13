## MODIFIED Requirements

### Requirement: env Lambda is fast (no boot)
The `spinloop remote env` command SHALL NOT trigger an instance boot. It reads the API key from Secrets Manager, the base URL from the environment's Elastic IP, and — best-effort — the environment's deploy-config from SSM; none of these require the instance to be running or booting.

#### Scenario: env does not start a stopped instance
- **WHEN** the user runs `spinloop remote env` and the instance is stopped
- **THEN** the command returns quickly with an error (not after minutes of booting)

## ADDED Requirements

### Requirement: env Lambda reports what is deployed
The `env` Lambda SHALL read the named environment's deploy-config from SSM (the same state `deploy` writes and `start`/`stats` already relay) and, when one is present and parses, include it in its reply alongside `base_url` and `api_key`: a `deployed` flag, the `runner`, the `modelId`, the `servedName` (the name the engine answers to — an `ALIAS` at deploy time, falling back to the model id), and the `contextSize`.

When no deploy-config is registered for the environment, or the stored value fails to parse, the reply SHALL omit these fields rather than fail the request: `base_url` and `api_key` remain valid and useful without them, exactly as they were before this requirement existed.

#### Scenario: env reports the deployed model
- **WHEN** the user runs `spinloop remote env --env dev-3` and a model has been deployed to `dev-3`
- **THEN** the reply includes `deployed: true`, the `runner`, `modelId`, `servedName` and `contextSize` the deploy recorded, alongside `base_url` and `api_key`

#### Scenario: env degrades gracefully with nothing deployed
- **WHEN** the user runs `spinloop remote env --env dev-3` and `dev-3` is registered but nothing has been deployed to it
- **THEN** the reply carries `base_url` and `api_key` as it always has, with no `deployed`, `runner`, `modelId`, `servedName` or `contextSize` fields, and the command does not fail because of their absence
