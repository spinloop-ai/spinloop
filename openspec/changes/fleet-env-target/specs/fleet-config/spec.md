## MODIFIED Requirements

### Requirement: Fleet file resolution

The `spinloop fleet` commands SHALL resolve the fleet they act on from the
flags given: `--env <name>` names a single registered cloud environment,
`--fleet <path>` names a fleet file, and with neither the fleet file is
`./fleet.yaml` in the working directory. A missing fleet file when one is
required SHALL fail with a message naming the expected path and how to create
one.

`--env` and `--fleet` SHALL NOT both be given: each names where the model is
served from, so a command stating both SHALL fail naming both rather than
resolving one by precedence. The same rule governs a launch, so an operator
meets it once.

`--env` SHALL name a registered environment, which is a plain identifier and
never a path. A value that is not a plain identifier, and a name with no
registered configuration, SHALL each fail saying so and how to create the
environment, before any node is contacted.

`--fleet` SHALL carry a `-f` short form on every `spinloop fleet` subcommand
except `fleet logs`, where `-f` is the short form of `--follow`; on `fleet
logs` the fleet file SHALL be named by the long form `--fleet` only. `--env`
SHALL carry no short form on the fleet commands, so that one spelling means
one thing across the group.

#### Scenario: Default resolution

- **WHEN** a `spinloop fleet` command runs in a directory containing
  `fleet.yaml` with no `--fleet` or `--env` flag
- **THEN** that file is used

#### Scenario: Explicit path

- **WHEN** `spinloop fleet status --fleet ./cluster.yaml` runs
- **THEN** that file is used

#### Scenario: Short form

- **WHEN** `spinloop fleet status -f ./cluster.yaml` runs
- **THEN** `./cluster.yaml` is used, exactly as with `--fleet`

#### Scenario: logs keeps -f for follow

- **WHEN** the operator runs `spinloop fleet logs -f`
- **THEN** that is the command's follow flag, not a fleet-file flag, and
  `logs` takes its fleet file only as `--fleet`

#### Scenario: Missing file

- **WHEN** a `spinloop fleet` command runs with no fleet file at the resolved
  path
- **THEN** it fails, naming the expected path

#### Scenario: A named environment is the target

- **WHEN** `spinloop fleet status --env prod` runs
- **THEN** the command acts on the registered environment `prod` alone,
  whether or not a `fleet.yaml` is present

#### Scenario: A named environment needs no fleet file

- **WHEN** `spinloop fleet dashboard --env prod` runs in a directory holding
  no `fleet.yaml`
- **THEN** the view opens on that one environment, and the absent file is not
  an error

#### Scenario: Naming both a fleet and an environment fails

- **WHEN** `spinloop fleet status --env prod --fleet ./cluster.yaml` runs
- **THEN** it fails naming both the environment and the fleet file, and
  contacts nothing

#### Scenario: The directory's fleet file does not conflict with --env

- **WHEN** `spinloop fleet status --env prod` runs in a directory containing
  `fleet.yaml`
- **THEN** the environment is the target and the directory's file is ignored,
  because only a flag states a conflict

#### Scenario: An unregistered environment is reported

- **WHEN** `spinloop fleet status --env nope` runs and no configuration is
  registered under that name
- **THEN** it fails saying the environment is not registered and how to
  create it, and contacts nothing

#### Scenario: A path is not an environment name

- **WHEN** `spinloop fleet status --env ./remote.json` runs
- **THEN** it fails saying an environment name is a plain identifier with no
  path

## ADDED Requirements

### Requirement: A registered environment is a fleet of one

A `--env <name>` target SHALL be a fleet holding exactly one node: a cloud
node named by the flag, whose registered configuration is the one
`remotes/<name>/remote.json` holds. It SHALL be driven, observed and rendered
exactly as the same node listed in a fleet file is, so a command's output for
one environment does not depend on how that environment was named.

Such a fleet has no file on disk. It SHALL therefore carry none of the
settings a fleet file supplies — no activity preference, no wake policy, no
gateway, no concurrency limits, no fleet-wide key reference — and SHALL take
each of those as its default. Where a command reports which fleet it acted
on, it SHALL name the environment rather than a path.

A token reference SHALL NOT be resolved for such a fleet: a cloud node is
reached through its control plane with the operator's own credentials, and
there is no file for an adjacent `.env` to sit beside. An environment whose
credentials are missing or expired SHALL fail as it does on any other path.

#### Scenario: One environment renders as one node

- **WHEN** `spinloop fleet status --env prod` runs
- **THEN** the output is the one-row status table a single-node fleet file
  produces for the same environment

#### Scenario: An environment that cannot be reached is a row

- **WHEN** `spinloop fleet status --env prod` runs and the environment cannot
  be reached
- **THEN** the row reports why, and the command still succeeds, exactly as an
  unreachable node in a fleet file does

#### Scenario: Fleet-wide settings are absent, not inherited

- **WHEN** a command acts on a `--env` target
- **THEN** no activity preference, wake policy, gateway or concurrency limit
  is in force, and a `fleet.yaml` in the working directory supplies none of
  them

#### Scenario: No .env is consulted

- **WHEN** a command acts on a `--env` target
- **THEN** no adjacent `.env` is read, because the target names no file to sit
  beside
