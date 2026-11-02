# Spec Delta

## MODIFIED Requirements

### Requirement: Cloud commands load the Spinloop's local environment

The `cloud` control commands that resolve a Spinloop — `deploy`, `start`,
`stop`, `status`, `stats`, and the other subcommands that take one — SHALL,
before performing any AWS or control-plane work, load environment variables
into the process environment from two sources tied to the resolved Spinloop:
the `.env` file beside that Spinloop, and the Spinloop's
own `ENV` instructions. The loaded values SHALL be visible to everything the
command does afterwards — the AWS credential chain, the region resolution, and
the `SPINLOOP_CLOUD_*` overrides. The Spinloop is read for these local values
only: which environment the command acts on is selected by the command's
`--node` flag, with the `default` environment as the fallback, and the Spinloop
itself names no environment. When a command resolves no Spinloop (no path
argument and no `./Spinloop`), there is nothing adjacent to load and the command
SHALL proceed on the process environment alone. When the resolved Spinloop was
fetched from a URL, there is likewise no local `.env` beside it to load — a
URL has no local directory — and the command SHALL proceed on the Spinloop's own
`ENV` instructions and the process environment alone, exactly as the
no-Spinloop-resolved case does.

#### Scenario: A .env beside the Spinloop reaches the control calls

- **WHEN** `spinloop cloud status ./Spinloop` runs and a `.env` beside that Spinloop
  sets `SPINLOOP_CLOUD_START_URL`, unset in the process environment
- **THEN** the command uses that value when it contacts the control endpoint

#### Scenario: Every control command loads the local environment

- **WHEN** any `cloud` subcommand resolves a Spinloop
- **THEN** that Spinloop's adjacent `.env` and its `ENV` instructions are loaded
  before the command performs any AWS or control-plane work

#### Scenario: No Spinloop, nothing to load

- **WHEN** a `cloud` command runs with no Spinloop resolved and acts on the
  `default` environment
- **THEN** no adjacent `.env` is read and the command proceeds on the process
  environment alone

#### Scenario: A URL-sourced Spinloop has no adjacent .env

- **WHEN** a `cloud` command resolves a Spinloop fetched from a URL
- **THEN** no `.env` read is attempted, and only that Spinloop's own `ENV`
  instructions and the process environment are applied
