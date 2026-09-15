## MODIFIED Requirements

### Requirement: Environment selection for remote commands

The endpoint's control URLs SHALL come from a JSON configuration naming a start
URL, a stop URL, an optional deploy URL, and a region. That configuration MAY
also name the endpoint's own base URL; it SHALL be optional, since no control
call needs it, and a configuration without it SHALL remain valid.

A `remote` subcommand SHALL select which environment's configuration it uses
with its `--env <name>` flag, and the flag SHALL be required: the value is a
registered environment's name, and the configuration is read from that
environment's `remote.json` in the per-user registry (see the Remote
Environments specification). A `--env` value that names an environment with no
registered configuration, and no complete configuration in the environment
variables, SHALL fail saying the environment is not registered and how to
create it.

A subcommand given no `--env` SHALL fail naming the flag and listing the
registered environments, and SHALL act on nothing. There is no environment a
command falls back to: several of these subcommands change the state of a
cloud instance, and an instance nobody named is not one to start, stop or
terminate. The name `default` is an ordinary environment name, carrying no
special meaning.

The Spinloop a subcommand is given as an argument SHALL NOT select an
environment; it SHALL be read only for its `ENV` instructions and the `.env`
file beside it, which the command applies before any AWS or control-plane work
(see the Remote Local Environment specification).

Environment variables SHALL override individual values, and the region SHALL
fall back to the standard AWS region variable and then to the region named in
the URL. A missing or incomplete configuration SHALL fail saying where to put
it.

#### Scenario: The flag selects the environment

- **WHEN** the user runs `spinloop remote status --env qwen3.6-27b-prod`
- **THEN** the URLs come from that environment's `remote.json` in the registry

#### Scenario: An unregistered environment is named as such

- **WHEN** a `remote` subcommand runs with `--env missing` and no environment
  `missing` is registered
- **THEN** it fails saying the environment is not registered and that
  `spinloop remote deploy --env missing` creates it

#### Scenario: No flag uses the default environment

- **WHEN** a `remote` subcommand runs with no `--env` flag
- **THEN** the `default` environment is used, whether or not a `Spinloop` is
  present in the working directory

#### Scenario: An explicit Spinloop does not select an environment

- **WHEN** a `remote` subcommand is given a Spinloop as its argument and no
  `--env` flag
- **THEN** the command uses the `default` environment and applies the
  Spinloop's `ENV` instructions and adjacent `.env` to the process environment,
  rather than failing for the Spinloop to name an environment

#### Scenario: Configuration without a base URL

- **WHEN** a remote configuration names the control URLs and region but no base
  URL, and a `remote` subcommand runs
- **THEN** the subcommand works as it always has, since the endpoint reports its
  own address in the replies to `start` and `status`

#### Scenario: A command with no environment names the flag

- **WHEN** the user runs a `remote` subcommand with no `--env`
- **THEN** it fails naming `--env` and listing the registered environments,
  and contacts nothing

#### Scenario: An instance is never stopped without being named

- **WHEN** the user runs `spinloop remote stop` with no `--env`, with an
  environment named `default` registered
- **THEN** nothing is stopped: the command fails naming the flag, and
  `default` is not assumed

#### Scenario: default is an ordinary name

- **WHEN** the user runs a `remote` subcommand with `--env default` and that
  environment is registered
- **THEN** it acts on that environment, exactly as it would for any other name
