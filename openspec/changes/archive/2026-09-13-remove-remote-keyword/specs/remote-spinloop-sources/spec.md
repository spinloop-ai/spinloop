# Delta: remote-spinloop-sources

## MODIFIED Requirements

### Requirement: Relative-reference resolution

A relative reference SHALL resolve against the reference that named it,
matching the kind of that base reference: a relative reference under a local
base SHALL resolve as a filesystem path joined against the base's own
directory; a relative reference under a URL base SHALL resolve the way a
relative link resolves against a base document (the base's own last path
segment is dropped, exactly as `net/url`'s reference resolution defines).
A reference that is already absolute — an absolute local path, or a URL —
SHALL be used unchanged regardless of what named it, so a local Spinloop MAY
name a `PRESET` that is itself a URL, and a URL-sourced Spinloop MAY
name one that is an absolute local path.

#### Scenario: Relative reference under a local base

- **WHEN** a local Spinloop at `/home/user/proj/Spinloop` names `PRESET
  ./preset.ini`
- **THEN** the preset resolves to `/home/user/proj/preset.ini`

#### Scenario: Relative reference under a URL base

- **WHEN** a Spinloop fetched from `https://example.com/team/Spinloop` names
  `PRESET ./preset.ini`
- **THEN** the preset resolves to `https://example.com/team/preset.ini`

#### Scenario: Absolute URL reference regardless of base

- **WHEN** a local Spinloop names `PRESET https://example.com/preset.ini`
- **THEN** the preset resolves to that URL unchanged, and is fetched over HTTP

#### Scenario: Absolute local reference under a URL base

- **WHEN** a Spinloop fetched from a URL names `PRESET /opt/shared/preset.ini`
- **THEN** the preset resolves to that local path unchanged, and is read from
  local disk

### Requirement: Fetching happens only at the point of use

Resolving or parsing a Spinloop SHALL NOT, by itself, fetch anything a `PRESET`
reference names. It SHALL be fetched only when a command that actually
consumes it does so, at the same point in the command's existing flow that a
local-path reference would be read from disk.

#### Scenario: Reading a Spinloop does not fetch its PRESET

- **WHEN** a Spinloop naming a `PRESET` (local or remote) is parsed for any
  purpose, including `spinloop apply`, which never consumes `PRESET`
- **THEN** the `PRESET` reference is not fetched

#### Scenario: A command fetches only the reference it needs

- **WHEN** `spinloop serve` runs against a Spinloop whose `PRESET` is a URL
- **THEN** only the `PRESET` is fetched
