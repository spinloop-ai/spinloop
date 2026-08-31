## ADDED Requirements

### Requirement: Remote commands load the Spinloop's local environment

The `remote` control commands — `deploy`, `start`, `stop`, and `status` — SHALL,
before resolving remote configuration or performing any AWS or control-plane
work, load environment variables into the process environment from two sources
tied to the resolved Spinloop: the `.env` file beside that Spinloop, and the Spinloop's
own `ENV` instructions. The loaded values SHALL be visible to everything the
command does afterwards — the AWS credential chain, the region resolution, and
the `SPINLOOP_REMOTE_*` overrides. When a command resolves no Spinloop (no path
argument and no `./Spinloop`), there is nothing adjacent to load and the command
SHALL proceed on the process environment alone.

#### Scenario: A .env beside the Spinloop reaches the control calls

- **WHEN** `spinloop remote status ./Spinloop` runs and a `.env` beside that Spinloop
  sets `SPINLOOP_REMOTE_START_URL`, unset in the process environment
- **THEN** the command uses that value when it contacts the control endpoint

#### Scenario: Every control command loads the local environment

- **WHEN** any of `deploy`, `start`, `stop`, or `status` resolves a Spinloop
- **THEN** that Spinloop's adjacent `.env` and its `ENV` instructions are loaded
  before the command performs any AWS or control-plane work

#### Scenario: No Spinloop, nothing to load

- **WHEN** a `remote` command runs with no Spinloop resolved and falls back to the
  per-user configuration
- **THEN** no adjacent `.env` is read and the command proceeds on the process
  environment alone

### Requirement: Precedence of local environment sources

When the same variable is set in more than one source, the value SHALL be chosen
with the precedence, highest to lowest: the Spinloop's `ENV` instruction, then a
variable already present in the process environment, then the `.env` beside the
Spinloop. A variable already set in the process environment SHALL therefore win
over the `.env`, which only fills gaps; an `ENV` instruction SHALL override both.

#### Scenario: The process environment wins over .env

- **WHEN** a variable is set both in the process environment and in the `.env`
  beside the Spinloop
- **THEN** the process environment's value is used and the `.env` value is
  ignored

#### Scenario: .env fills a gap

- **WHEN** a variable is set in the `.env` beside the Spinloop and is unset in the
  process environment
- **THEN** the `.env` value is used

#### Scenario: ENV overrides both

- **WHEN** a variable is set by a Spinloop `ENV` instruction and also in the
  process environment and/or the `.env`
- **THEN** the `ENV` value is used, overriding both

### Requirement: ENV is local-only

Variables established from the Spinloop's `ENV` instructions (and its adjacent
`.env`) SHALL affect only the local device invoking `spinloop`. They SHALL NOT be
included in the deploy payload sent to the instance, nor otherwise transmitted to
the deployed instance.

#### Scenario: Deploy does not forward ENV to the instance

- **WHEN** `spinloop remote deploy` runs for a Spinloop that declares `ENV` variables
- **THEN** the configuration sent to the deploy endpoint contains none of those
  variables — they shape only the local command's own environment
