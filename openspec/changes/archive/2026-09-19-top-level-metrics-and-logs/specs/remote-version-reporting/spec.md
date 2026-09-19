## REMOVED Requirements

### Requirement: Remote status shows version

**Reason**: `spinloop remote status` is removed — one command reports an
engine's state, whether it is named as an environment or as a fleet node.
The behaviour is unchanged and is restated below under a name that does not
carry the removed command's spelling.

**Migration**: `spinloop status --env <name>`.

### Requirement: Remote metrics shows version

**Reason**: `spinloop remote metrics` is removed; the version it displayed is
displayed by the command that replaced it, from the same stats reply. Restated
below under a name that does not carry the removed spelling.

**Migration**: `spinloop metrics --env <name>`.

## ADDED Requirements

### Requirement: An environment's metrics show version

`spinloop metrics --env <name>` SHALL display the spinloop version in its output, as the stats Lambda already reads the daemon and can carry the version alongside its existing fields.

#### Scenario: Version is shown in table format

- **WHEN** the user runs `spinloop metrics --env <name> --format=table` against a running instance
- **THEN** the table output includes a `version` line

#### Scenario: Version is shown in JSON format

- **WHEN** the user runs `spinloop metrics --env <name> --format=json` against a running instance
- **THEN** the JSON output includes a `version` field

#### Scenario: Version is omitted from bar header when unavailable

- **WHEN** the user runs `spinloop metrics --env <name> --format=bar` and the version is not available
- **THEN** the bar header omits the version without error

### Requirement: An environment's status shows version

`spinloop status --env <name>` SHALL display the spinloop version running on the remote instance alongside its existing state, health, and base URL fields.

#### Scenario: Version is shown when the instance is running

- **WHEN** the user runs `spinloop status --env <name>` against a running instance
- **THEN** the output includes a `version` line with the spinloop version string (e.g. `version: 1.16.0`)

#### Scenario: Version is unavailable when the instance is stopped

- **WHEN** the user runs `spinloop status --env <name>` against a stopped instance
- **THEN** the output omits the version line, since the daemon is not reachable
