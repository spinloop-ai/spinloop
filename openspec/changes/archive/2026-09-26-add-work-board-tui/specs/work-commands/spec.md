## MODIFIED Requirements

### Requirement: The work commands as work list clients

`spinloop work` SHALL be a top-level command group with the subcommands
add, list, abort, remove, logs and board, each a client of the
orchestrator's work list API. Every subcommand SHALL take a `--url` flag
naming the API's base address, and SHALL present the API's token as a
bearer on every request it makes — resolved from `--api-token`, else
`--api-token-file`, else the `SPINLOOP_API_TOKEN` environment variable,
two of the flags given at once being a refusal naming both. A subcommand
that names no `--url` SHALL fail before it calls the API, naming the flag.
The commands SHALL be clients of the API alone: they SHALL NOT read or
write the items file, the state, or the logs directly, and SHALL NOT take
any lock beside them.

#### Scenario: The API's address is named

- **WHEN** the operator runs a work command naming the API's address with
  `--url`
- **THEN** it calls that API, presenting the token as a bearer

#### Scenario: No address is named

- **WHEN** the operator runs a work command with no `--url`
- **THEN** it fails, naming the `--url` flag, and calls no API

#### Scenario: Two token flags at once

- **WHEN** the operator gives both `--api-token` and `--api-token-file`
- **THEN** the command fails, naming both flags, and calls no API

#### Scenario: The token comes from the environment

- **WHEN** the operator names the API's address, sets no token flag, and the
  `SPINLOOP_API_TOKEN` environment is set
- **THEN** the command presents that value as the bearer
