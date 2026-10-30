# Spec Delta

## ADDED Requirements

### Requirement: A shortcut for harness open with node targeting
`spinloop code` SHALL launch the active harness exactly as
`spinloop harness open` does: it SHALL accept the same flags — `-O`/`--spinloop`,
`-n`/`--node`, `--fleet`, `--prefer`, `--no-wake`, `--wake-timeout`,
`-H`/`--harness`, and `--providers` — apply a Spinloop the same way, and forward
every argument it does not consume to the harness untouched. A leading argument
naming a registered alias or a path, the `-O`/`--spinloop` flag, and the
`--node` target all drive the same Spinloop application as on `harness open`.
`spinloop code <args>` SHALL therefore produce the same harness config, launch
the same process, and print the same output and errors as
`spinloop harness open <args>`. `code` SHALL NOT change what `harness open`
does; it is a new spelling of it, not a replacement. A word on the command
line that is neither a consumed Spinloop name nor a known flag forwards to the
harness as ever — except the removed `--env`/`-e`, which the command SHALL
reject naming `--node` as its replacement, since forwarding the launch's own
former flag would hand the harness a word it never owned.

#### Scenario: the removed --env spelling names its replacement

- **WHEN** the user runs `spinloop code --env dev-2`
- **THEN** the command fails naming `--env` and `--node` as its replacement,
  and the harness is not launched with `--env` among its arguments

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

#### Scenario: --node configures from a deployed environment

- **WHEN** the user runs `spinloop code --node dev-2` with no Spinloop
- **THEN** the harness is configured from what is deployed to `dev-2` and
  launched, as `spinloop harness open --node dev-2` would do

#### Scenario: Trailing arguments are forwarded untouched

- **WHEN** the user runs `spinloop code -- --agent-flag`
- **THEN** the harness is launched and `--agent-flag` is forwarded to it, as
  `spinloop harness open -- --agent-flag` would do

## REMOVED Requirements

### Requirement: A shortcut for harness open
**Reason**: The requirement enumerates the launch flags and names the target flag in a scenario title; the flag is renamed `--env`/`-e` to `--node`/`-n`, and a renamed flag cannot be carried by a MODIFIED whose scenario titles the validator pins to the old spelling.
**Migration**: The requirement "A shortcut for harness open with node targeting" states the same contract with the new spelling.
