# Delta: lucinate-integration

## MODIFIED Requirements

### Requirement: Config location and shape

The lucinate adapter SHALL read and write lucinate's connections store at
`connections.json` under lucinate's data directory — `$LUCINATE_DATA_DIR` when
set, otherwise `~/.lucinate` (resolved from the home directory, **not** XDG). The
store is plain JSON of the form
`{"defaultId": "<id>", "connections": [{id, name, type, url, defaultModel, …}]}`.
A missing file SHALL be treated as an empty store, and the directory SHALL be
created when needed. The file SHALL be written with owner-only (`0600`)
permissions.

#### Scenario: First write creates the store

- **WHEN** the user runs `spinloop harness add -H lucinate` and no
  `connections.json` exists
- **THEN** the file is created with the managed connection and owner-only
  permissions

#### Scenario: The data directory is overridden

- **WHEN** `LUCINATE_DATA_DIR` is set and the lucinate harness writes its config
- **THEN** the store is written under that directory, not under `~/.lucinate`

### Requirement: Preserving merge

Writes SHALL merge only the managed connection: other connections in the store,
the ordering of unrelated entries, and any unknown fields — on the store or on
the managed connection — SHALL round-trip untouched. When the managed connection
already exists, its creation timestamp SHALL be preserved and only the fields
spinloop owns (`type`, `url`, `defaultModel`, `name`) SHALL be overwritten.

#### Scenario: Sibling connections survive

- **WHEN** the store already holds another connection and the user runs
  `spinloop harness add -H lucinate`
- **THEN** that connection is intact afterwards

#### Scenario: Unknown fields round-trip

- **WHEN** the managed connection already carries fields spinloop does not own
  (for example a last-used timestamp or a future field)
- **THEN** those fields are preserved after a re-apply

### Requirement: Removing a connection

Removing a provider SHALL delete the managed connection spinloop created for it.
When the removed connection was the store's `defaultId`, that field SHALL be
cleared so lucinate falls back to its own startup selection. Other connections
SHALL be untouched. The operation SHALL report how many entries it removed.

#### Scenario: Remove deletes the managed connection

- **WHEN** the user runs `spinloop harness remove -H lucinate` for a provider
  previously applied
- **THEN** the managed connection is gone and other connections remain

#### Scenario: Removing the default clears the pointer

- **WHEN** the removed connection was named by `defaultId`
- **THEN** `defaultId` is cleared afterwards

### Requirement: Reading state back

The adapter SHALL read the store back for `spinloop harness show` and
`spinloop harness export`, reporting each managed connection as a configured
provider: its model key (from `defaultModel`) and its base URL (from `url`).
Because a lucinate connection has no fields for context or output limits, the
adapter SHALL report none, and those limits SHALL NOT round-trip through
`export`. lucinate has no single top-level default *model* setting distinct
from the connection, so the read-back SHALL report no top-level default model.

#### Scenario: Export reconstructs provider and model

- **WHEN** `spinloop harness export -H lucinate` runs against a store with a
  managed connection
- **THEN** the reconstructed Spinloop names that provider, its model, and its
  base URL

#### Scenario: Limits do not round-trip

- **WHEN** a selection with a context window is applied to lucinate and then
  exported
- **THEN** the exported Spinloop carries no context or output limit, because the
  connection cannot hold them
