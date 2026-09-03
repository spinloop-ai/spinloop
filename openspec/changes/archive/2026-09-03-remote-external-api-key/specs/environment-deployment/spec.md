## ADDED Requirements

### Requirement: Externally provided API key

`spinloop remote deploy` SHALL accept an externally provided API key as a
reference to an environment variable, and pass it to the control plane to store
as the environment's API key. The value SHALL NOT be written on the command line
or in any file the CLI owns: the flag names a variable, and the CLI resolves it
from the process environment — which the Spinloop's local environment has already
populated — before sending it. A named variable that is set nowhere SHALL fail
the deploy, naming the variable, before anything is sent.

The key SHALL travel in the signed deploy request as a **request-scoped** field
— riding beside the allowed CIDR and the reseed choice — and SHALL NOT be part
of the persisted deploy-config and SHALL NOT appear in any reply. The control
plane SHALL store a supplied key in the environment's existing API-key secret:
creating the secret when it is absent, and setting its value when the secret
already exists.

Deploying with a key is a rotation: it SHALL replace the environment's secret,
instantly invalidating the previous key. Deploying without a key SHALL leave any
existing secret untouched and SHALL NOT regenerate one. A deploy that supplies a
key SHALL report that the key was applied — or, when it replaced an existing
one, that it was rotated — without printing the value, so an operator who
rotates a key out from under a live agent sees it happen rather than discovering
it as 401s.

#### Scenario: A deploy stores a supplied key

- **WHEN** `spinloop remote deploy` is given a key variable that is set, for an
  environment whose API-key secret does not yet exist
- **THEN** the environment's API-key secret is created holding that value, and
  the report says the key was applied

#### Scenario: A deploy rotates an existing key

- **WHEN** `spinloop remote deploy` is given a key for an environment that already
  has an API-key secret
- **THEN** the secret is set to the new value, the old key is no longer valid,
  and the report says the key was rotated

#### Scenario: A deploy without a key keeps the existing one

- **WHEN** `spinloop remote deploy` runs with no key for an environment that
  already has an API-key secret
- **THEN** the secret is left unchanged and no new key is generated

#### Scenario: The key is not persisted or echoed

- **WHEN** a deploy supplies a key
- **THEN** the value is not written to the environment's deploy-config, is not
  in the registered remote configuration, and is not printed in any reply or in
  the deploy report

#### Scenario: A named variable that is unset fails early

- **WHEN** `spinloop remote deploy` names a key variable that is set nowhere
- **THEN** the deploy fails naming the variable, before anything is sent
