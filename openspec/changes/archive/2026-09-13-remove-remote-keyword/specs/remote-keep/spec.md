# Delta: remote-keep

## ADDED Requirements

### Requirement: The keep command selects its environment

The `keep` subcommand SHALL select which environment's instance to tag using the same rule as the other `remote` subcommands: the `--env <name>` flag naming a registered environment, or the per-user default environment.

#### Scenario: Keep names the environment with the flag

- **WHEN** the user runs `spinloop remote keep 2h --env dev-2`
- **THEN** the `dev-2` environment's instance has its tag set

#### Scenario: Keep falls back to the default environment

- **WHEN** the user runs `spinloop remote keep 2h` with no `--env` flag
- **THEN** the `default` environment's instance is tagged

## REMOVED Requirements

### Requirement: The keep command resolves the environment like other remote subcommands

**Reason**: The resolution ran through a Spinloop's `REMOTE` instruction; the
instruction is removed and the `--env` flag selects the environment instead.

**Migration**: Use `spinloop remote keep <duration> --env <name>`; with no flag
the `default` environment is tagged.
