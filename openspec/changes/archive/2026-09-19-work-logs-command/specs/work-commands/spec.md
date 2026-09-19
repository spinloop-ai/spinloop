## MODIFIED Requirements

### Requirement: The work commands as work list clients

`spinloop work` SHALL be a top-level command group with the subcommands add,
list, abort, remove and logs, each a client of the orchestrator's work list
API. Every subcommand SHALL take a `--url` flag naming the API's base
address, and SHALL present the API's token as a bearer on every request it
makes — resolved from `--api-token`, else `--api-token-file`, else the
`SPINLOOP_API_TOKEN` environment variable, two of the flags given at once
being a refusal naming both. A subcommand that names no `--url` SHALL fail
before it calls the API, naming the flag. The commands SHALL be clients of
the API alone: they SHALL NOT read or write the items file, the state, or
the logs directly, and SHALL NOT take any lock beside them.

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

## ADDED Requirements

### Requirement: Reading an item's log through the work list API

`spinloop work logs <id>` SHALL call the API's `GET /v1/items/{id}/log`
path for the item's kept agent output and print it. An item the API
answers as carrying no output yet SHALL be printed as empty, not treated
as a fault. An id the run does not carry SHALL be refused, naming it, the
way the API states it.

Given `-f`/`--follow`, the command SHALL instead poll the API for new
output and print it as it arrives, the way `tail -f` does: it SHALL keep
polling while the item's state is `backlog` or `running` — an item named
before, or just as, it starts is still followed — and SHALL stop once the
API reports the item `done` or `failed`, printing whatever output arrived
up to that point first. The operator's interrupt SHALL also end a follow,
cleanly. The command SHALL NOT itself read the items file, the state, or
the logs directly: the API's answers are the whole source of what it
prints and when it stops.

#### Scenario: An item's kept output is printed

- **WHEN** the operator runs `work logs` naming an item with kept output
- **THEN** the command prints it

#### Scenario: An item with no output yet is empty, not a fault

- **WHEN** the operator runs `work logs` naming an item the API answers
  with no output yet
- **THEN** the command prints nothing, and does not fail

#### Scenario: An id the run does not carry is refused

- **WHEN** the operator runs `work logs` naming an id the API does not
  carry
- **THEN** the command fails, naming the id, the way the API states it

#### Scenario: A follow streams new output as it arrives

- **WHEN** the operator runs `work logs -f` on a running item, and its
  agent produces more output
- **THEN** the command prints the new output as later polls see it

#### Scenario: A follow waits through the backlog

- **WHEN** the operator runs `work logs -f` on an item the run has not yet
  started
- **THEN** the command keeps polling rather than ending, and starts
  printing output once the item runs

#### Scenario: A follow ends once the item ends

- **WHEN** a followed item's state becomes `done` or `failed`
- **THEN** the command prints whatever output arrived up to that point and
  ends

#### Scenario: A follow ends on the operator's interrupt

- **WHEN** the operator interrupts a running follow
- **THEN** the command ends cleanly, without reporting a failure
