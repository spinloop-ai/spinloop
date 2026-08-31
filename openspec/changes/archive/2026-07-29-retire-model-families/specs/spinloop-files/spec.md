## MODIFIED Requirements

### Requirement: Spinloop file format

A Spinloop SHALL be a flat, line-oriented text file of `KEYWORD value`
instructions. The keywords are `PROVIDER`, `MODEL`, `ALIAS`, `CONTEXT`,
`OUTPUT`, `BASEURL` (also accepted as `BASE-URL`, `BASE_URL`, or `URL`),
`PRESET`, and `REMOTE`. Keywords SHALL match case-insensitively, with UPPERCASE
as the canonical form. Blank lines, full-line `#` comments, and trailing
comments introduced by whitespace-then-`#` SHALL be ignored. Each instruction
SHALL take exactly one value and SHALL appear at most once. `PROVIDER` is
required. Parse errors SHALL name the offending line.

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

#### Scenario: Missing provider

- **WHEN** a Spinloop has no `PROVIDER` instruction
- **THEN** parsing fails saying the PROVIDER instruction is missing

#### Scenario: Naming a remote endpoint

- **WHEN** a Spinloop contains `REMOTE ./remote.json`
- **THEN** it parses, and the value is available to the `remote` command group

### Requirement: Applying and unapplying a Spinloop

`spinloop apply` SHALL apply the Spinloop's selection exactly as the equivalent
`spinloop add` would, and `spinloop unapply` SHALL remove what the Spinloop selects
exactly as the equivalent `spinloop remove` would. A command-line `--output`/`-o`
on `apply` SHALL override the Spinloop's `OUTPUT` instruction, and `--providers`
SHALL override the catalogue it resolves against (a Spinloop never names a
catalogue). `apply` SHALL ignore a `PRESET` instruction — it is consumed only
by `spinloop serve`.

#### Scenario: Apply equals add

- **WHEN** a Spinloop with `PROVIDER ollama` and `MODEL llama3.2` is applied
- **THEN** the harness config matches what `spinloop add -p ollama -m llama3.2`
  would have produced

#### Scenario: Output override

- **WHEN** a Spinloop sets `OUTPUT 32k` and the user runs
  `spinloop apply --output 16k`
- **THEN** the applied output limit is 16000 tokens

#### Scenario: Preset is not apply's business

- **WHEN** a Spinloop with a `PRESET` instruction is applied
- **THEN** the harness config is written as if the instruction were absent

### Requirement: Exporting the current config

`spinloop export` SHALL reconstruct a canonical Spinloop from the active harness's
config and print it to stdout. The provider exported is chosen by the
`--provider`/`-p` flag, else the default model's provider, else the sole
configured provider; with several providers and no way to choose, the command
SHALL fail listing them. The output SHALL name the configured model with a
`MODEL` instruction, SHALL omit a `BASEURL` that only restates the catalogue's
default, and SHALL record `CONTEXT`/`OUTPUT` only when the exported models agree
on a single value — never inventing one. Rendered output SHALL use canonical
UPPERCASE keywords with aligned values, so `spinloop export > Spinloop` round-trips.

#### Scenario: Round-trip through export

- **WHEN** the user applies a Spinloop and then runs `spinloop export`
- **THEN** the printed Spinloop selects the same provider, model, and limits

#### Scenario: Ambiguous provider

- **WHEN** several providers are configured, none is the default model's, and
  no `-p` is given
- **THEN** the command fails listing the configured providers to choose from

#### Scenario: Nothing configured

- **WHEN** the harness config has no providers
- **THEN** the command fails naming the config file it read
