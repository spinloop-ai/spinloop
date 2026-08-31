## ADDED Requirements

### Requirement: Keys reach the launched agent

When spinloop launches a harness, the agent's environment SHALL carry the API key
variables spinloop can resolve for the catalogue's providers, so a key kept where
only spinloop reads it still reaches the agent. Neither harness stores the secret
itself — each resolves a reference when it runs — so without this the user would
have to set the variable by hand. A variable already present in spinloop's own
environment SHALL be passed through unchanged, so an explicit setting always
wins. Failure to read the catalogue SHALL NOT prevent the launch.

#### Scenario: A key only spinloop can see still reaches the agent

- **WHEN** spinloop can resolve a provider's key variable but it is absent from
  the environment, and the harness is launched
- **THEN** the launched agent's environment carries that variable

#### Scenario: An explicit setting is not overridden

- **WHEN** the variable is already set in the environment and spinloop can also
  resolve a different value
- **THEN** the launched agent sees the environment's value

#### Scenario: An unreadable catalogue still launches the agent

- **WHEN** the provider catalogue cannot be loaded
- **THEN** the harness is launched anyway, with the environment unchanged
