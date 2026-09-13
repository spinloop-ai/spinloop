# Delta: spinloop-files

## MODIFIED Requirements

### Requirement: Spinloop file format

A Spinloop SHALL be a flat, line-oriented text file of `KEYWORD value`
instructions. The keywords are `PROVIDER`, `MODEL`, `ALIAS`, `CONTEXT`,
`OUTPUT`, `PARALLEL`, `BASEURL` (also accepted as `BASE-URL`, `BASE_URL`, or
`URL`), `PRESET`, and `ENV`. Keywords SHALL match
case-insensitively, with UPPERCASE as the canonical form. Blank lines, full-line
`#` comments, and trailing comments introduced by whitespace-then-`#` SHALL be
ignored. Each instruction SHALL take exactly one value; every instruction SHALL
appear at most once, except `ENV`, which MAY be repeated. An `ENV` instruction's
value SHALL be a single `KEY=VALUE` token with a non-empty key and no
whitespace. `PARALLEL`'s value SHALL name a count of concurrent request slots;
like `CONTEXT`, its numeric validity (a positive integer) is enforced by the
commands that consume it rather than by parsing itself, and what it does to
the served engine's command is defined by the `local-serving` capability.
`PROVIDER` is required. Parse errors SHALL name the offending line.

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

#### Scenario: Naming a remote endpoint

- **WHEN** a Spinloop contains `REMOTE ./remote.json` on any line
- **THEN** parsing fails on that line, since the `REMOTE` instruction was
  removed: the environment is now named with `remote deploy --env <name>` at
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
  the command that goes on to use it (`serve`, `remote deploy`, a fleet wake)
  fails naming the value, rather than silently treating it as a slot count

## ADDED Requirements

### Requirement: REMOTE is a removed keyword

A `REMOTE` instruction SHALL be rejected at parse time with an error that names
the offending line, says the `REMOTE` instruction was removed, and states the
replacement: name the environment with `remote deploy --env <name>` at deploy
time, and pass `--env <name>` to the commands that act on the environment
(`remote` subcommands, `apply`, `unapply`, `harness`). The error SHALL NOT be
the generic unknown-keyword message.

#### Scenario: A REMOTE line is rejected with the migration message

- **WHEN** a Spinloop contains `REMOTE ./remote.json` on line 3
- **THEN** parsing fails citing line 3, saying `REMOTE` was removed, and naming
  `--env` as the way to name the environment

#### Scenario: The generic error is not used for REMOTE

- **WHEN** a Spinloop contains a `REMOTE` line
- **THEN** the error names the removal and its replacement rather than merely
  listing the accepted keywords
