## MODIFIED Requirements

### Requirement: Stats subcommand

The system SHALL provide a `metrics` subcommand (`spinloop remote metrics`) that reports the current state of a remote inference instance. It SHALL accept the same Spinloop resolution as `start`, `stop`, and `deploy` — an optional positional Spinloop path, defaulting to `./Spinloop` when present — and SHALL require the Spinloop to name a `REMOTE` environment. When the instance is running, the report SHALL include the spinloop version from the daemon, carried by the stats Lambda reply.

#### Scenario: Stats with a running instance

- **WHEN** the user runs `spinloop remote metrics` with a running instance
- **THEN** the command reports the instance state, runner, model, spinloop version, GPU info, CPU/RAM usage, token counts, and request counts

#### Scenario: Stats with a stopped instance

- **WHEN** the user runs `spinloop remote metrics` and the instance is stopped
- **THEN** the command reports `state: stopped` and no metrics

#### Scenario: Stats resolves the Spinloop

- **WHEN** the user runs `spinloop remote metrics` in a directory with an `Spinloop` that has a `REMOTE` instruction
- **THEN** the command uses that Spinloop's remote environment without an explicit path argument

#### Scenario: Stats with explicit Spinloop path

- **WHEN** the user runs `spinloop remote metrics ./some/Spinloop`
- **THEN** the command uses that Spinloop's `REMOTE` environment

#### Scenario: Version is shown in stats output

- **WHEN** the user runs `spinloop remote metrics` with a running instance
- **THEN** the output includes the spinloop version
