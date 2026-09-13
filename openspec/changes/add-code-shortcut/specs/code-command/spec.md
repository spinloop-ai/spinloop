# Code Command Specification

## Purpose

`spinloop code`: the one-word command that launches the active harness — a
shortcut for `spinloop harness open` — so the most common action, "start my
configured coding agent", is as short as `spinloop up`. It takes the same
arguments, applies a Spinloop the same way, and launches the same harness,
dispatching to `harness open` rather than reimplementing it.

## ADDED Requirements

### Requirement: A shortcut for harness open

`spinloop code` SHALL launch the active harness exactly as
`spinloop harness open` does: it SHALL accept the same flags — `-O`/`--spinloop`,
`--env`, `--fleet`, `--node`, `--prefer`, `--no-wake`, `--wake-timeout`,
`-H`/`--harness`, and `--providers` — apply a Spinloop the same way, and forward
every argument it does not consume to the harness untouched. A leading argument
naming a registered alias or a path, the `-O`/`--spinloop` flag, and the `--env`
environment all drive the same Spinloop application as on `harness open`.
`spinloop code <args>` SHALL therefore produce the same harness config, launch
the same process, and print the same output and errors as
`spinloop harness open <args>`. `code` SHALL NOT change what `harness open`
does; it is a new spelling of it, not a replacement.

#### Scenario: A bare code launches unconfigured

- **WHEN** the user runs `spinloop code` with no arguments
- **THEN** the active harness is launched with no Spinloop applied, exactly as
  `spinloop harness open` would launch it

#### Scenario: A leading alias configures then launches

- **WHEN** the user runs `spinloop code qwen3.6-27b -- --agent-flag` and
  `qwen3.6-27b` is a registered alias
- **THEN** the aliased Spinloop is applied first and the harness is launched,
  forwarding `--agent-flag`, as `spinloop harness open qwen3.6-27b -- --agent-flag`
  would do

#### Scenario: A valueless -O wears the directory's Spinloop

- **WHEN** the user runs `spinloop code -O` in a directory holding a `Spinloop`
- **THEN** `./Spinloop` is applied and the harness launches, as
  `spinloop harness open -O` would do

#### Scenario: --env configures from a deployed environment

- **WHEN** the user runs `spinloop code --env dev-2` with no Spinloop
- **THEN** the harness is configured from what is deployed to `dev-2` and
  launched, as `spinloop harness open --env dev-2` would do

#### Scenario: Trailing arguments are forwarded untouched

- **WHEN** the user runs `spinloop code -- --agent-flag`
- **THEN** the harness is launched and `--agent-flag` is forwarded to it, as
  `spinloop harness open -- --agent-flag` would do

### Requirement: Completion offers the Spinloop slot

`code`'s tab completion SHALL offer the Spinloop slot — registered alias names
plus paths — exactly as `spinloop harness open`'s first positional slot does.
An unreadable config, an unloadable catalogue, or nonsense input SHALL leave it
silent: no candidates, no error, and nothing written to stderr.

#### Scenario: The Spinloop slot completes

- **WHEN** the user completes `spinloop code <TAB>`
- **THEN** registered alias names and paths are offered, as on
  `spinloop harness open <TAB>`

#### Scenario: A broken config stays quiet

- **WHEN** spinloop's config file is unreadable and completion is attempted on
  `spinloop code`
- **THEN** no candidates are offered, the command exits zero, and nothing is
  written to stderr
