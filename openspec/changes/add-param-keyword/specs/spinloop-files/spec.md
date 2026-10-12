## MODIFIED Requirements

### Requirement: Spinloop file format

A Spinloop SHALL be a flat, line-oriented text file of `KEYWORD value`
instructions. The keywords are `PROVIDER`, `MODEL`, `ALIAS`, `CONTEXT`,
`OUTPUT`, `PARALLEL`, `BASEURL` (also accepted as `BASE-URL`, `BASE_URL`, or
`URL`), `PRESET`, `PARAM`, and `ENV`. Keywords SHALL match
case-insensitively, with UPPERCASE as the canonical form. Blank lines, full-line
`#` comments, and trailing comments introduced by whitespace-then-`#` SHALL be
ignored. Each instruction SHALL take exactly one value; every instruction SHALL
appear at most once, except `ENV` and `PARAM`, which MAY be repeated. An `ENV`
instruction's value SHALL be a single `KEY=VALUE` token with a non-empty key and
no whitespace. A `PARAM` instruction's value SHALL be `key = value`, split at
the first `=`: the key SHALL be a single token with no whitespace, and the value
MAY be empty or contain whitespace. A PARAM names one engine flag in the
engine's own vocabulary — what it does to the served engine's command is
defined by the `local-serving` capability — and where the same key is set by
several PARAM lines the later line's value SHALL stand. `PARALLEL`'s value SHALL
name a count of concurrent request slots; like `CONTEXT`, its numeric validity
(a positive integer) is enforced by the commands that consume it rather than by
parsing itself, and what it does to the served engine's command is defined by
the `local-serving` capability. `PROVIDER` is required. Parse errors SHALL name
the offending line.

#### Scenario: A minimal Spinloop

- **WHEN** a file containing only `PROVIDER openrouter` and
  `MODEL deepseek/deepseek-v4-pro` is parsed
- **THEN** it yields a selection of that provider and model

#### Scenario: Duplicate instruction

- **WHEN** a Spinloop sets `MODEL` on two lines
- **THEN** parsing fails, citing both line numbers

#### Scenario: Unknown keyword

- **WHEN** a Spinloop contains `HARNESS pi`
- **THEN** parsing fails listing the accepted keywords

#### Scenario: Naming a fleet

- **WHEN** a Spinloop contains `FLEET ./fleet.yaml`
- **THEN** parsing fails as an unknown keyword, listing the accepted keywords,
  and no special migration message is printed

#### Scenario: Missing provider

- **WHEN** a Spinloop has no `PROVIDER` instruction
- **THEN** parsing fails saying the PROVIDER instruction is missing

#### Scenario: Naming a cloud endpoint

- **WHEN** a Spinloop contains `REMOTE ./remote.json` on any line
- **THEN** parsing fails on that line, since the `REMOTE` instruction was
  removed: the environment is now named with `cloud deploy --env <name>` at
  deploy time and `--env <name>` at the commands that act on it

#### Scenario: Declaring local environment variables

- **WHEN** a Spinloop contains `ENV AWS_PROFILE=dev` and `ENV AWS_REGION=eu-west-2`
  on separate lines
- **THEN** it parses, yielding both key/value pairs in the selection, and the
  repetition is not treated as a duplicate-instruction error

#### Scenario: Malformed ENV value

- **WHEN** a Spinloop contains an `ENV` instruction whose value has no `=` or an
  empty key
- **THEN** parsing fails, naming the offending line

#### Scenario: Setting the number of parallel slots

- **WHEN** a Spinloop contains `PARALLEL 2`
- **THEN** it parses, yielding a parallel count of 2 in the selection

#### Scenario: A non-numeric or non-positive PARALLEL is caught on use

- **WHEN** a Spinloop contains `PARALLEL 0`, `PARALLEL -1`, or `PARALLEL abc`
- **THEN** parsing accepts the raw value, exactly as it does for `CONTEXT`, and
  the command that goes on to use it (`serve`, `cloud deploy`, a fleet wake)
  fails naming the value, rather than silently treating it as a slot count

#### Scenario: Declaring engine parameters

- **WHEN** a Spinloop contains `PARAM jinja = 1` and
  `PARAM hf = unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL # the weights` on separate
  lines
- **THEN** it parses, yielding both entries in the selection, and the repetition
  is not treated as a duplicate-instruction error

#### Scenario: A PARAM may repeat a key

- **WHEN** a Spinloop sets `PARAM ngl = 99` on one line and `PARAM ngl = 97` on
  a later line
- **THEN** it parses, and the later line's value is the one that stands

#### Scenario: Malformed PARAM value

- **WHEN** a Spinloop contains a `PARAM` instruction whose value has no `=`, an
  empty key, or a key containing whitespace
- **THEN** parsing fails, naming the offending line

### Requirement: Applying and unapplying a Spinloop

`spinloop harness apply` SHALL apply the Spinloop's selection exactly as the equivalent
`spinloop harness add` would, and `spinloop harness unapply` SHALL remove what the Spinloop selects
exactly as the equivalent `spinloop harness remove` would. A command-line `--output`/`-o`
on `apply` SHALL override the Spinloop's `OUTPUT` instruction, and `--providers`
SHALL override the catalogue it resolves against (a Spinloop never names a
catalogue). `apply` SHALL ignore a `PRESET` instruction — it is consumed only
by `spinloop serve` — and SHALL ignore `PARAM` instructions the same way: they
name engine flags that only a serve or a deploy consumes.

#### Scenario: Apply equals add

- **WHEN** a Spinloop with `PROVIDER ollama` and `MODEL llama3.2` is applied
- **THEN** the harness config matches what `spinloop harness add -p ollama -m llama3.2`
  would have produced

#### Scenario: Output override

- **WHEN** a Spinloop sets `OUTPUT 32k` and the user runs
  `spinloop harness apply --output 16k`
- **THEN** the applied output limit is 16000 tokens

#### Scenario: Preset is not apply's business

- **WHEN** a Spinloop with a `PRESET` instruction is applied
- **THEN** the harness config is written as if the instruction were absent

#### Scenario: Params are not apply's business

- **WHEN** a Spinloop with `PARAM` instructions is applied
- **THEN** the harness config is written as if the instructions were absent
