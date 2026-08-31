## MODIFIED Requirements

### Requirement: Remote commands load the Spinloop's local environment

The `remote` control commands that resolve a Spinloop — `deploy`, `start`,
`stop`, `status`, and `stats` — SHALL,
before resolving remote configuration or performing any AWS or control-plane
work, load environment variables into the process environment from two sources
tied to the resolved Spinloop: the `.env` file beside that Spinloop, and the Spinloop's
own `ENV` instructions. The loaded values SHALL be visible to everything the
command does afterwards — the AWS credential chain, the region resolution, and
the `SPINLOOP_REMOTE_*` overrides. When a command resolves no Spinloop (no path
argument and no `./Spinloop`), there is nothing adjacent to load and the command
SHALL proceed on the process environment alone. When the resolved Spinloop was
fetched from a URL, there is likewise no local `.env` beside it to load — a
URL has no local directory — and the command SHALL proceed on the Spinloop's own
`ENV` instructions and the process environment alone, exactly as the
no-Spinloop-resolved case does.

#### Scenario: A .env beside the Spinloop reaches the control calls

- **WHEN** `spinloop remote status ./Spinloop` runs and a `.env` beside that Spinloop
  sets `SPINLOOP_REMOTE_START_URL`, unset in the process environment
- **THEN** the command uses that value when it contacts the control endpoint

#### Scenario: Every control command loads the local environment

- **WHEN** any of `deploy`, `start`, `stop`, `status`, or `stats` resolves an
  Spinloop
- **THEN** that Spinloop's adjacent `.env` and its `ENV` instructions are loaded
  before the command performs any AWS or control-plane work

#### Scenario: No Spinloop, nothing to load

- **WHEN** a `remote` command runs with no Spinloop resolved and falls back to the
  per-user configuration
- **THEN** no adjacent `.env` is read and the command proceeds on the process
  environment alone

#### Scenario: A URL-sourced Spinloop has no adjacent .env

- **WHEN** a `remote` command resolves a Spinloop fetched from a URL
- **THEN** no `.env` read is attempted, and only that Spinloop's own `ENV`
  instructions and the process environment are applied
