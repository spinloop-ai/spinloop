# Spinloop Files Specification

## ADDED Requirements

### Requirement: Environment references in values

A `PARAM` instruction's value and the value of an entry in the file named by a
`PRESET` instruction SHALL support environment references: `$NAME` and
`${NAME}` expand to the variable's value, `${NAME:-default}` expands to the
variable's value unless it is unset or empty, in which case it expands to
`default`, and `$$` renders a literal `$`. An unset or empty variable with no
default expands to the empty string. `NAME` is a shell-style identifier: a
letter or underscore followed by letters, digits or underscores. A `$` that
does not begin one of these forms — and is not `$$` — SHALL stay literal, and
an empty default, `${NAME:-}`, SHALL be valid.

Parsing a Spinloop SHALL NOT resolve references: `Parse`, `show` and `export`
carry and render the raw text, so re-rendering a file never writes a resolved
value anywhere. Resolution belongs to the command that consumes the value —
`spinloop serve`, `spinloop cloud deploy`, a node wake — against the process
environment as that command holds it at the moment it builds its configuration.
Resolving preset values SHALL happen when the preset is read by such a command,
never when the Spinloop naming it is merely read or applied.

A reference the syntax does not define — an unterminated `${`, an invalid or
empty name inside `${}`, or any other `${...}` form such as `${NAME:+x}` —
SHALL fail the consuming command with an error naming the offending preset
line or PARAM key, rather than passing the raw text to the engine. A value with
no references SHALL be unaffected, exactly as before this requirement existed.

Preset entry keys and PARAM keys are flag names and SHALL NOT be resolved.

#### Scenario: A parameter value takes its machine's path

- **WHEN** `MODEL_DIR` is set to `/models` and a Spinloop states
  `PARAM m = ${MODEL_DIR}/qwen.gguf`, and `spinloop serve` builds its command
- **THEN** the command carries `/models/qwen.gguf`

#### Scenario: An unset variable falls back to its default

- **WHEN** `HF_TAG` is unset and a Spinloop states
  `PARAM hf = org/model:${HF_TAG:-Q4_K_M}`, and `spinloop serve` builds its
  command
- **THEN** the command carries `org/model:Q4_K_M`

#### Scenario: An unset variable with no default expands to nothing

- **WHEN** `HF_TAG` is unset and a preset entry states `hf = org/model:$HF_TAG`
- **THEN** the resolved value is `org/model:`

#### Scenario: A doubled dollar is a literal dollar

- **WHEN** a Spinloop states `PARAM price = $$5` and `spinloop serve` builds
  its command
- **THEN** the command carries the value `$5`

#### Scenario: A stray dollar stays literal

- **WHEN** a value contains `$` followed by a character that cannot begin a
  name (e.g. `$5` or `100%$`)
- **THEN** resolution leaves that `$` exactly as written

#### Scenario: Parsing keeps the raw text

- **WHEN** a Spinloop states `PARAM hf = org/model:$HF_TAG` and is parsed or
  exported
- **THEN** the selection and the rendered file carry the raw
  `org/model:$HF_TAG`, with nothing resolved

#### Scenario: A malformed reference names its preset line

- **WHEN** a preset read by `spinloop serve` contains `temp = ${1FOO}` on line
  4
- **THEN** the command fails naming line 4, rather than passing
  `${1FOO}` to the engine

#### Scenario: A malformed reference names its PARAM key

- **WHEN** a Spinloop states `PARAM hf = org/model:${HF_TAG` and
  `spinloop serve` builds its command
- **THEN** the command fails naming the PARAM's key `hf`, rather than passing
  the raw text to the engine

#### Scenario: Keys are not resolved

- **WHEN** a preset entry states `${FOO} = 1`
- **THEN** the entry key is the literal text `${FOO}`, rendered as a flag
  spelled from it, not the variable's value
