# Delta: provider-catalog

## ADDED Requirements

### Requirement: Provider command group

A `spinloop provider` group SHALL hold the catalogue commands: `list` and
`init`, each carrying the flags, arguments, and output its former top-level
spelling had (the old `init-providers` is renamed `init`, since the parent
already says provider). A bare `spinloop provider` SHALL show the group's help,
as the `fleet` and `remote` groups do.

The former top-level spellings of the two catalogue commands SHALL
NOT exist. Invoking one SHALL fail with an error that names the command's new
home — giving `spinloop provider list` for `list` and `spinloop provider init`
for `init-providers` — rather than the generic unknown-command message.

#### Scenario: A subcommand runs under provider

- **WHEN** the user runs `spinloop provider list`
- **THEN** the catalogue is printed exactly as the former top-level `list` did
  before the move

#### Scenario: The old top-level spelling names its new home

- **WHEN** the user runs the former top-level spelling `list`
- **THEN** the command fails, saying list has moved under provider and giving
  `spinloop provider list`

#### Scenario: Bare provider shows its help

- **WHEN** the user runs `spinloop provider` with no argument
- **THEN** the group's help is shown, naming `list` and `init`

## MODIFIED Requirements

### Requirement: Catalogue listing

`spinloop provider list` SHALL print every provider in the catalogue in stable
(alphabetical) order, showing for each: its id and description, its API key
environment variable (marked `(required)` when the key is mandatory), and the
harnesses that support it (`opencode`, plus `pi` when the provider has a `pi`
block).

#### Scenario: Listing the built-in catalogue

- **WHEN** the user runs `spinloop provider list`
- **THEN** every embedded provider is printed with its key requirements and
  supported harnesses

### Requirement: Catalogue scaffolding

`spinloop provider init [path]` SHALL write a copy of the embedded
catalogue to `./providers.yaml` (or the given path) as a starting point for
customisation, and SHALL refuse to overwrite an existing file unless
`--force`/`-F` is given. On success it SHALL print how to point `spinloop` at
the written file.

#### Scenario: Refuses to clobber an existing file

- **WHEN** the user runs `spinloop provider init` and `./providers.yaml`
  already exists
- **THEN** the command fails, telling the user to pass a different path or
  `--force`

#### Scenario: Writing the catalogue out

- **WHEN** the user runs `spinloop provider init custom.yaml` and no such
  file exists
- **THEN** the embedded catalogue is written to `custom.yaml` byte-for-byte
