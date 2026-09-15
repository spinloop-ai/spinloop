## MODIFIED Requirements

### Requirement: Fleet file resolution

Every command that acts on a fleet — the `spinloop fleet` group, and the
top-level verbs that read one — SHALL resolve the fleet the same way, from the
flags given: `--env <name>` names a single registered cloud environment,
`--fleet <path>` names a fleet file, and with neither the fleet file is
`./fleet.yaml` in the working directory. One rule serves every such command, so
a target means the same thing wherever it is given.

Where no target resolves — no `--env`, no `--fleet`, and no fleet file at the
expected path — the command SHALL fail naming every way to give one: the
expected path, `--fleet <path>`, and `--env <name>`. It SHALL NOT look for an
engine on the machine it is running on. A local engine is already shown by the
command that runs it, and a machine whose engines are to be read this way names
them in a fleet file like any other.

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

#### Scenario: No target at all names every way to give one

- **WHEN** a command that acts on a fleet runs with no `--env`, no `--fleet`,
  and no fleet file at the expected path
- **THEN** it fails naming the expected path, `--fleet <path>` and
  `--env <name>`, and contacts nothing

#### Scenario: No engine on this machine is looked for

- **WHEN** a read verb runs with no target resolvable on a machine that is
  running an engine with its control API up
- **THEN** it fails as above rather than reporting that engine: a local engine
  is read through the command that runs it, or by naming the machine in a fleet
  file
