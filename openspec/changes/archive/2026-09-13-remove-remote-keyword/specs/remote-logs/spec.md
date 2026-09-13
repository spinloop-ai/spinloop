# Delta: remote-logs

## MODIFIED Requirements

### Requirement: An environment's shipped logs are readable from the CLI

`spinloop remote logs` SHALL print the logs an environment's instances have
shipped, without the operator needing to know the log group or stream naming,
open the AWS console, or connect to an instance. It SHALL select which
environment to read using the same rules as the other remote subcommands — the
`--env <name>` flag naming a registered environment, else the `default`
environment — so `spinloop remote logs` and `spinloop remote status` given the
same `--env` always speak about the same environment.

#### Scenario: Reading the current environment's logs

- **WHEN** the operator runs `spinloop remote logs` where `spinloop remote status`
  would report on an environment
- **THEN** the log events that environment's instances shipped are printed
- **AND** the operator is not required to name a log group, stream, or instance

#### Scenario: Reading a named environment's logs

- **WHEN** the operator runs `spinloop remote logs --env dev-2`
- **THEN** `dev-2`'s logs are printed rather than the default
  environment's
