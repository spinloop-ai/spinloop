# Delta: remote-stats

## ADDED Requirements

### Requirement: Metrics subcommand

The system SHALL provide a `metrics` subcommand (`spinloop remote metrics`) that reports the current state of a remote inference instance. It SHALL select which environment it reports on using the same rule as the other `remote` subcommands: the `--env <name>` flag naming a registered environment, falling back to the `default` environment when the flag is absent. A Spinloop given as an argument is read only for its `ENV` instructions and adjacent `.env`, never to select the environment. When the instance is running, the report SHALL include the spinloop version from the daemon, carried by the stats Lambda reply.

#### Scenario: Stats with a running instance

- **WHEN** the user runs `spinloop remote metrics` with a running instance
- **THEN** the command reports the instance state, runner, model, spinloop version, GPU info, CPU/RAM usage, token counts, and request counts

#### Scenario: Stats with a stopped instance

- **WHEN** the user runs `spinloop remote metrics` and the instance is stopped
- **THEN** the command reports `state: stopped` and no metrics

#### Scenario: Stats names the environment with the flag

- **WHEN** the user runs `spinloop remote metrics --env dev-2`
- **THEN** the command reports on the `dev-2` environment's instance

#### Scenario: Stats falls back to the default environment

- **WHEN** the user runs `spinloop remote metrics` with no `--env` flag
- **THEN** the command reports on the `default` environment's instance

#### Scenario: Version is shown in stats output

- **WHEN** the user runs `spinloop remote metrics` with a running instance
- **THEN** the output includes the spinloop version

## REMOVED Requirements

### Requirement: Stats subcommand

**Reason**: The subcommand required a Spinloop naming a `REMOTE` environment;
the instruction is removed and the `--env` flag selects the environment
instead.

**Migration**: Use `spinloop remote metrics --env <name>`; with no flag the
`default` environment is reported on.
