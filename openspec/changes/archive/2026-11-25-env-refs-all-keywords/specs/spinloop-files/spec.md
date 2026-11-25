# Spec Delta: spinloop-files

## MODIFIED Requirements

### Requirement: Environment references in values

The value of every Spinloop instruction SHALL support environment
references: `$NAME` and `${NAME}` expand to the variable's value,
`${NAME:-default}` expands to the variable's value unless it is unset or
empty, in which case it expands to `default`, and `$$` renders a literal
`$`. An unset or empty
variable with no default expands to the empty string. `NAME` is a
shell-style identifier: a letter or underscore followed by letters, digits
or underscores. A `$` that does not begin one of these forms — and is not
`$$` — SHALL stay literal, and an empty default, `${NAME:-}`, SHALL be
valid. The values that carry references are the values of every keyword
instruction — `PROVIDER`, `MODEL`, `ALIAS`, `CONTEXT`, `OUTPUT`, `PARALLEL`,
`BASEURL` and `PRESET` — the value half of each `ENV` instruction's
`KEY=VALUE`, each `PARAM` instruction's value, and the value of an entry in
the file named by a `PRESET` instruction.

Parsing a Spinloop SHALL NOT resolve references: `Parse`, `show` and
`export` carry and render the raw text, so re-rendering a file never writes
a resolved value anywhere. Resolution belongs to the command that consumes
the value — a serve, an apply or unapply, a harness launch, an alias
registration, `spinloop cloud deploy`, a node wake, and any other command
that acts on the file's values — against the process environment as that
command holds it at the moment it builds its configuration. Resolving
preset values SHALL happen when the preset is read by such a command, never
when the Spinloop naming it is merely parsed, shown or exported. Where such
a command first layers the Spinloop's `.env` and `ENV` instructions onto
its environment, every reference resolves against that layered environment,
so any value may reference what the Spinloop's own `ENV` contributes; an
`ENV` value carrying a reference SHALL itself resolve — against the
environment as layered so far — before its variable is set.

Where a value is interpreted by another grammar, the resolved text is what
is interpreted: a `CONTEXT`, `OUTPUT` or `PARALLEL` is validated as a size
or count only after resolution, a `PROVIDER` value resolves before the
catalogue lookup it drives, and a `PRESET` value resolves before its path
is resolved against the Spinloop's own source.

Instruction keywords, `PARAM` keys, `ENV` keys and preset entry keys are
names and SHALL NOT be resolved.

A reference the syntax does not define — an unterminated `${`, an invalid
or empty name inside `${}`, or any other `${...}` form such as
`${NAME:+x}` — SHALL fail the consuming command with an error naming the
offending instruction: by its keyword, by its `PARAM` key, or by the
offending preset line. A value with no references SHALL be unaffected,
exactly as before this requirement existed.

#### Scenario: A parameter value takes its machine's path

- **WHEN** `MODEL_DIR` is set to `/models` and a Spinloop states
  `PARAM m = ${MODEL_DIR}/qwen.gguf`, and `spinloop serve` builds its
  command
- **THEN** the command carries `/models/qwen.gguf`

#### Scenario: A model value carries the machine's tag

- **WHEN** `HF_TAG` is set to `Q4_K_M` and a Spinloop states
  `MODEL org/model:$HF_TAG`, and `spinloop serve` builds its command
- **THEN** the command carries `org/model:Q4_K_M`

#### Scenario: A base URL reaches the harness resolved

- **WHEN** `HOST` is set to `gpu.internal` and a Spinloop states
  `BASEURL http://${HOST:-localhost}:8080/v1`, and `spinloop harness apply`
  writes the provider block
- **THEN** the written block carries `http://gpu.internal:8080/v1`, and the
  Spinloop file still reads `http://${HOST:-localhost}:8080/v1`

#### Scenario: A size takes its value from a default

- **WHEN** `CTX` is unset and a Spinloop states `CONTEXT ${CTX:-32k}`, and
  `spinloop serve` builds its command
- **THEN** the command carries the size 32000 resolves to, rather than the
  command failing to parse `${CTX:-32k}` as a size

#### Scenario: The provider is looked up after resolution

- **WHEN** `ENGINE` is set to `llamacpp` and a Spinloop states
  `PROVIDER $ENGINE`, and `spinloop serve` runs
- **THEN** the engine named by the catalogue under `llamacpp` is served

#### Scenario: A preset path arrives from a variable

- **WHEN** `PRESETS_DIR` is set to the directory holding a Spinloop's
  `preset.ini` and the Spinloop states `PRESET ${PRESETS_DIR}/preset.ini`,
  and `spinloop serve` reads the preset
- **THEN** the file at the resolved path is read, and the narration names
  that path

#### Scenario: An alias registers the resolved name

- **WHEN** `NICK` is set to `qwen` and a Spinloop's only name is
  `ALIAS ${NICK}`, and `spinloop alias` registers it with no `--name`
- **THEN** the entry is registered under `qwen`

#### Scenario: Unapply removes what apply wrote

- **WHEN** a Spinloop stating `MODEL org/model:${TAG}` is applied while
  `TAG` holds `v1` and unapplied while `TAG` still holds `v1`
- **THEN** the block apply wrote — keyed on the resolved `org/model:v1` — is
  the block unapply removes

#### Scenario: An ENV value resolves before it is set

- **WHEN** `SLOT_BASE` is unset and a Spinloop states `ENV SLOT=${SLOT_BASE:-1}`,
  and a command that layers the Spinloop's environment runs
- **THEN** the process environment holds `SLOT=1`, and a value referencing
  `$SLOT` resolves to `1` on that path

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

- **WHEN** a Spinloop states `PARAM hf = org/model:$HF_TAG`,
  `MODEL org/model:$HF_TAG` and `BASEURL http://$HOST/v1`, whatever the
  environment holds, and is parsed or exported
- **THEN** the selection and the rendered file carry the raw
  `org/model:$HF_TAG` and `http://$HOST/v1`, with nothing resolved

#### Scenario: A malformed reference names its preset line

- **WHEN** a preset read by `spinloop serve` contains `temp = ${1FOO}` on
  line 4
- **THEN** the command fails naming line 4, rather than passing
  `${1FOO}` to the engine

#### Scenario: A malformed reference names its PARAM key

- **WHEN** a Spinloop states `PARAM hf = org/model:${HF_TAG` and
  `spinloop serve` builds its command
- **THEN** the command fails naming the PARAM's key `hf`, rather than
  passing the raw text to the engine

#### Scenario: A malformed reference names its keyword

- **WHEN** a Spinloop states `MODEL org/model:${HF_TAG` and
  `spinloop serve` builds its command
- **THEN** the command fails naming the `MODEL` instruction, rather than
  passing the raw text to the engine

#### Scenario: Keys are not resolved

- **WHEN** a preset entry states `${FOO} = 1` and a Spinloop states
  `ENV ${FOO}=1`
- **THEN** the preset entry key is the literal text `${FOO}`, rendered as a
  flag spelled from it, and the `ENV` variable's name is the literal
  `${FOO}` — neither takes the variable's value
