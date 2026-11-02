# Spec Delta

## MODIFIED Requirements

### Requirement: An environment's status shows version

`spinloop status --node <name>` SHALL display the spinloop version running on the cloud instance alongside its existing state, health, and base URL fields.

#### Scenario: Version is shown when the instance is running

- **WHEN** the user runs `spinloop status --node <name>` against a running instance
- **THEN** the output includes a `version` line with the spinloop version string (e.g. `version: 1.16.0`)

#### Scenario: Version is unavailable when the instance is stopped

- **WHEN** the user runs `spinloop status --node <name>` against a stopped instance
- **THEN** the output omits the version line, since the daemon is not reachable

### Requirement: An environment's metrics show version

`spinloop metrics --node <name>` SHALL display the spinloop version in its output, as the stats Lambda already reads the daemon and can carry the version alongside its existing fields.

#### Scenario: Version is shown in table format

- **WHEN** the user runs `spinloop metrics --node <name> --format=table` against a running instance
- **THEN** the table output includes a `version` line

#### Scenario: Version is shown in JSON format

- **WHEN** the user runs `spinloop metrics --node <name> --format=json` against a running instance
- **THEN** the JSON output includes a `version` field

#### Scenario: Version is omitted from bar header when unavailable

- **WHEN** the user runs `spinloop metrics --node <name> --format=bar` and the version is not available
- **THEN** the bar header omits the version without error
