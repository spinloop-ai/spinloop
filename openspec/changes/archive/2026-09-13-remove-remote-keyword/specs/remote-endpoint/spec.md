# Delta: remote-endpoint

## ADDED Requirements

### Requirement: Environment selection for remote commands

The endpoint's control URLs SHALL come from a JSON configuration naming a start
URL, a stop URL, an optional deploy URL, and a region. That configuration MAY
also name the endpoint's own base URL; it SHALL be optional, since no control
call needs it, and a configuration without it SHALL remain valid.

A `remote` subcommand SHALL select which environment's configuration it uses
with its `--env <name>` flag: the value is a registered environment's name, and
the configuration is read from that environment's `remote.json` in the per-user
registry (see the Remote Environments specification). A `--env` value that names
an environment with no registered configuration SHALL fail saying the
environment is not registered and how to create it. When no `--env` flag is
given, the `default` environment SHALL be used, so the command works outside
any project.

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

## REMOVED Requirements

### Requirement: Remote configuration discovery

**Reason**: The `REMOTE` instruction is removed from the Spinloop grammar, and
with it the path and URL forms of a remote configuration; the environment is
selected by the `--env` flag instead.

**Migration**: Pass `--env <name>` to the `remote` subcommands; with no flag
the `default` environment is used. A path- or URL-form configuration is no
longer addressable; the `SPINLOOP_REMOTE_*` overrides carry a manual
configuration where one is needed.
