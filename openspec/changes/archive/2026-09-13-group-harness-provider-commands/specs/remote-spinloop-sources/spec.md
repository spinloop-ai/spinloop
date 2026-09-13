# Delta: remote-spinloop-sources

## MODIFIED Requirements

### Requirement: Fetching happens only at the point of use

Resolving or parsing a Spinloop SHALL NOT, by itself, fetch anything a `PRESET`
reference names. It SHALL be fetched only when a command that actually
consumes it does so, at the same point in the command's existing flow that a
local-path reference would be read from disk.

#### Scenario: Reading a Spinloop does not fetch its PRESET

- **WHEN** a Spinloop naming a `PRESET` (local or remote) is parsed for any
  purpose, including `spinloop harness apply`, which never consumes `PRESET`
- **THEN** the `PRESET` reference is not fetched

#### Scenario: A command fetches only the reference it needs

- **WHEN** `spinloop serve` runs against a Spinloop whose `PRESET` is a URL
- **THEN** only the `PRESET` is fetched
