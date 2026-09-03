## ADDED Requirements

### Requirement: Fleet-wide API key reference

A `fleet.yaml` MAY declare a top-level `apiKeyEnv` naming the environment
variable that holds the API key shared by the fleet's remote nodes. The file
SHALL hold the variable's *name*, never the value — the same discipline as the
daemon and engine-token references — and the reference SHALL be resolved exactly
the way those are: the process environment first, then the `.env` beside the
fleet file.

The reference is the default key for a remote node. A remote node whose own
entry names no `engineTokenEnv` takes the fleet-wide key. A node's own
`engineTokenEnv` SHALL override the fleet-wide reference, so one remote may
carry a distinct key while the rest of the fleet shares one. A daemon node SHALL
NOT take the fleet-wide reference: it is gated only by its own `engineTokenEnv`,
exactly as it is today.

A reference that is named but resolves to nothing SHALL be a configuration error
naming the variable, in the same way a missing engine-token variable is.

#### Scenario: A fleet shares one key across its remotes

- **WHEN** a `fleet.yaml` declares `apiKeyEnv: SHARED_KEY`, that variable is
  set, and it lists two `kind: remote` nodes that name no `engineTokenEnv`
- **THEN** the value of `SHARED_KEY` is the key both remotes are reached with

#### Scenario: A per-node reference overrides the fleet-wide one

- **WHEN** a `fleet.yaml` declares `apiKeyEnv: SHARED_KEY` and one remote node
  names `engineTokenEnv: SPECIAL_KEY`
- **THEN** that node is reached with the value of `SPECIAL_KEY` and the other
  remotes with the value of `SHARED_KEY`

#### Scenario: The fleet file holds no key value

- **WHEN** a `fleet.yaml` declaring `apiKeyEnv` is parsed
- **THEN** it carries only the reference, never a literal key value

#### Scenario: An unset fleet-wide variable names itself

- **WHEN** a `fleet.yaml` declares an `apiKeyEnv` that is set nowhere, and a
  remote node naming no key of its own is reached for its key
- **THEN** the failure names that variable, and no agent is launched without a
  key

#### Scenario: A daemon node is not gated by the fleet-wide key

- **WHEN** a `fleet.yaml` declares `apiKeyEnv` and a daemon node names no
  `engineTokenEnv`
- **THEN** the daemon node is started ungated, as it is today
