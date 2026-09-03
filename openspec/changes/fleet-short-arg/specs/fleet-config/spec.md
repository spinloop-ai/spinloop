## MODIFIED Requirements

### Requirement: Fleet file resolution

The `spinloop fleet` commands SHALL resolve the fleet file from an explicit
`--fleet <path>` when given, otherwise `./fleet.yaml` in the working
directory. A missing file when one is required SHALL fail with a message
naming the expected path and how to create one.

`--fleet` SHALL carry a `-f` short form on every `spinloop fleet` subcommand
except `fleet logs`, where `-f` is the short form of `--follow`; on `fleet
logs` the fleet file SHALL be named by the long form `--fleet` only.

#### Scenario: Default resolution

- **WHEN** a `spinloop fleet` command runs in a directory containing
  `fleet.yaml` with no `--fleet` flag
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
