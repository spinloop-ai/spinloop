# Delta: shell-completion

## MODIFIED Requirements

### Requirement: Completion surface coverage

Completion SHALL cover the full visible command surface, derived from the
registered command tree rather than a separate hand-maintained table: command
names (the hidden `__complete` excluded), each command's flags in both their
long and short forms, its subcommands where it has them, and context-aware
values — provider names from the resolved catalogue
(honouring a `--providers` override already on the line), harness names,
registered alias names where a Spinloop path is accepted, and the supported
shells for `completion`. The catalogue no longer enumerates models, so
`--model`/`-m` has no static candidate source; it SHALL still consume its value
so a following flag completes normally. For a command with subcommands, the
first positional slot SHALL offer those subcommands and any later slot SHALL fall
through to what the command otherwise accepts. Positional slots beyond a
command's arity SHALL offer nothing.

#### Scenario: Unalias offers exactly the registered names

- **WHEN** the user completes `spinloop unalias <TAB>`
- **THEN** the registered alias names are offered with no file paths

#### Scenario: New commands cannot be forgotten

- **WHEN** a new subcommand is added to the CLI's command tree
- **THEN** it is part of the completion surface automatically, and the guard
  test walks the command tree itself to verify, so there is no second table
  that could drift from the dispatch

#### Scenario: A nested command offers its subcommands

- **WHEN** the user completes `spinloop remote <TAB>`
- **THEN** its subcommands are offered, with no file paths

#### Scenario: After a subcommand, the Spinloop slot completes

- **WHEN** the user completes `spinloop remote deploy <TAB>`
- **THEN** registered alias names and paths are offered

#### Scenario: Providers complete from the catalogue

- **WHEN** the user completes `spinloop harness add -p <TAB>`
- **THEN** the catalogue's provider names are offered

#### Scenario: Flags not in a static table still complete

- **WHEN** a command registers a flag that is not named anywhere outside its
  own command definition and the user completes that command's flags
- **THEN** the flag is offered, in both its long and short forms

#### Scenario: The model flag has no static candidates but consumes its value

- **WHEN** the user completes `spinloop harness add -p openrouter -m <TAB>`
- **THEN** no model candidates are offered and no error occurs
- **AND** a flag typed after `--model <value>` still completes normally

#### Scenario: A group's first slot offers its subcommands

- **WHEN** the user completes `spinloop provider <TAB>`
- **THEN** `list` and `init` are offered, with no file paths

#### Scenario: The harness group's first slot offers only its subcommands

- **WHEN** the user completes `spinloop harness <TAB>` with no word typed
- **THEN** its subcommands — including `open` and `config` — are offered, with
  no Spinloop names, paths, or file paths

#### Scenario: open's first slot completes a Spinloop

- **WHEN** the user completes `spinloop harness open <TAB>`
- **THEN** registered alias names and paths are offered

#### Scenario: config --set offers harness names

- **WHEN** the user completes `spinloop harness config --set <TAB>`
- **THEN** the registered harness names are offered, with no file paths
