## REMOVED Requirements

### Requirement: Remote status shows version

**Reason**: `spinloop remote status` is removed; the version it displayed is
displayed by the command that replaced it. Restated below under a name that
does not carry the removed spelling.

**Migration**: `spinloop status --env <name>`.

### Requirement: Fleet status shows version

**Reason**: `spinloop fleet status` is removed in favour of the top-level
`spinloop status`, which serves every target kind. The per-node version
behaviour is unchanged and is restated below.

**Migration**: `spinloop status`.

## ADDED Requirements

### Requirement: An environment's status shows version

`spinloop status --env <name>` SHALL display the spinloop version running on the remote instance alongside its existing state, health, and base URL fields.

#### Scenario: Version is shown when the instance is running

- **WHEN** the user runs `spinloop status --env <name>` against a running instance
- **THEN** the output includes a `version` line with the spinloop version string (e.g. `version: 1.16.0`)

#### Scenario: Version is unavailable when the instance is stopped

- **WHEN** the user runs `spinloop status --env <name>` against a stopped instance
- **THEN** the output omits the version line, since the daemon is not reachable

### Requirement: Status shows version per node

`spinloop status` SHALL display the spinloop version for each node alongside its existing state and serving columns, read from the daemon's `/v1/status` response.

#### Scenario: Version is shown per node

- **WHEN** `spinloop status` runs against a fleet of running nodes
- **THEN** each node's row includes the spinloop version string

#### Scenario: Version is omitted for unreachable nodes

- **WHEN** a node's daemon is unreachable
- **THEN** that node's row shows its failure outcome without a version

#### Scenario: Versions differ across nodes

- **WHEN** nodes in the fleet run different spinloop versions
- **THEN** each node's row shows its own version, making the difference visible
