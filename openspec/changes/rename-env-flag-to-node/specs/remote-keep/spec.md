# Spec Delta

## MODIFIED Requirements

### Requirement: The keep command selects its environment

The `keep` subcommand SHALL select which environment's instance to tag using the same rule as the other `cloud` subcommands: the `--node <name>` flag naming a registered environment, or the per-user default environment.

#### Scenario: Keep names the environment with the flag

- **WHEN** the user runs `spinloop cloud keep 2h --node dev-2`
- **THEN** the `dev-2` environment's instance has its tag set

#### Scenario: Keep falls back to the default environment

- **WHEN** the user runs `spinloop cloud keep 2h` with no `--node` flag
- **THEN** the `default` environment's instance is tagged
