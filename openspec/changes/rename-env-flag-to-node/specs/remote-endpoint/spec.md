# Spec Delta

## MODIFIED Requirements

### Requirement: Environment selection for cloud commands

The endpoint's control URLs SHALL come from a JSON configuration naming a start
URL, a stop URL, an optional deploy URL, and a region. That configuration MAY
also name the endpoint's own base URL; it SHALL be optional, since no control
call needs it, and a configuration without it SHALL remain valid.

A `cloud` subcommand SHALL select which environment's configuration it uses
with its `--node <name>` flag, and the flag SHALL be required: the value is a
registered environment's name, and the configuration is read from that
environment's `cloud.json` in the per-user registry (see the Cloud
Environments specification). A `--node` value that names an environment with no
registered configuration, and no complete configuration in the environment
variables, SHALL fail saying the environment is not registered and how to
create it.

A subcommand given no `--node` SHALL fail naming the flag and listing the
registered environments, and SHALL act on nothing. There is no environment a
command falls back to: several of these subcommands change the state of a
cloud instance, and an instance nobody named is not one to start, stop or
terminate. The name `default` is an ordinary environment name, carrying no
special meaning.

The Spinloop a subcommand is given as an argument SHALL NOT select an
environment; it SHALL be read only for its `ENV` instructions and the `.env`
file beside it, which the command applies before any AWS or control-plane work
(see the Cloud Local Environment specification).

Environment variables SHALL override individual values, and the region SHALL
fall back to the standard AWS region variable and then to the region named in
the URL. A missing or incomplete configuration SHALL fail saying where to put
it.

#### Scenario: The flag selects the environment

- **WHEN** the user runs `spinloop cloud status --node qwen3.6-27b-prod`
- **THEN** the URLs come from that environment's `cloud.json` in the registry

#### Scenario: An unregistered environment is named as such

- **WHEN** a `cloud` subcommand runs with `--node missing` and no environment
  `missing` is registered
- **THEN** it fails saying the environment is not registered and that
  `spinloop cloud deploy --node missing` creates it

#### Scenario: No flag uses the default environment

- **WHEN** a `cloud` subcommand runs with no `--node` flag
- **THEN** the `default` environment is used, whether or not a `Spinloop` is
  present in the working directory

#### Scenario: An explicit Spinloop does not select an environment

- **WHEN** a `cloud` subcommand is given a Spinloop as its argument and no
  `--node` flag
- **THEN** the command uses the `default` environment and applies the
  Spinloop's `ENV` instructions and adjacent `.env` to the process environment,
  rather than failing for the Spinloop to name an environment

#### Scenario: Configuration without a base URL

- **WHEN** a cloud configuration names the control URLs and region but no base
  URL, and a `cloud` subcommand runs
- **THEN** the subcommand works as it always has, since the endpoint reports its
  own address in the replies to `start` and `status`

#### Scenario: A command with no environment names the flag

- **WHEN** the user runs a `cloud` subcommand with no `--node`
- **THEN** it fails naming `--node` and listing the registered environments,
  and contacts nothing

#### Scenario: An instance is never stopped without being named

- **WHEN** the user runs `spinloop cloud stop` with no `--node`, with an
  environment named `default` registered
- **THEN** nothing is stopped: the command fails naming the flag, and
  `default` is not assumed

#### Scenario: default is an ordinary name

- **WHEN** the user runs a `cloud` subcommand with `--node default` and that
  environment is registered
- **THEN** it acts on that environment, exactly as it would for any other name

### Requirement: An environment reports when it last did work

`spinloop status --node <name>` SHALL report how long it has been since the endpoint's
engine last did any work, alongside the instance state and health it reports
already. The figure SHALL come from the activity the on-instance daemon
tracks, not from a measurement the control plane makes itself — one answer,
derived on the box, however it is asked for.

The figure SHALL be labelled "last active", matching the wording and duration
formatting used everywhere else this fact appears, so the same fact reads the
same way in every command.

Collecting it SHALL NOT make `status` slower than its health check already
makes it: the daemon SHALL be asked in parallel with the health check rather
than after it. Nor SHALL it introduce a side effect — `status` SHALL remain a
read, and SHALL still perform no TCP probe.

#### Scenario: A running endpoint reports its last activity

- **WHEN** the user runs `spinloop status --node <name>` against a running endpoint
  whose engine has served work
- **THEN** the output reports how long ago that work happened, labelled "last
  active", beside the state and health lines

#### Scenario: Status stays a read

- **WHEN** the user runs `spinloop status --node <name>`
- **THEN** nothing is started, stopped or probed in order to obtain the
  last-active figure

### Requirement: An environment's status degrades when activity cannot be read

`spinloop status --node <name>` SHALL omit the last-active figure rather than fail,
report zero, or imply inactivity, whenever the figure cannot be obtained. That
covers an endpoint whose engine has not yet done any work, a daemon that
cannot be reached or answers unrecognisably, and an instance that is not
running — reaching the daemon needs a running box, so a stopped or undeployed
environment has nothing to report about its engine.

A failure to read the activity SHALL NOT affect the rest of the report: the
state and health lines SHALL be exactly what they are today, and the command
SHALL still succeed.

#### Scenario: A stopped instance reports no activity figure

- **WHEN** the user runs `spinloop status --node <name>` and the instance is stopped or
  undeployed
- **THEN** the output reports the state as it does today and shows no
  last-active figure

#### Scenario: An unreachable daemon does not spoil the report

- **WHEN** the endpoint is running but its daemon cannot be reached
- **THEN** the state and health lines are reported as they are today, no
  last-active figure is shown, and the command succeeds

#### Scenario: An engine that has done nothing yet

- **WHEN** the endpoint is running and its daemon reports no last-active time
- **THEN** no last-active figure is shown, rather than one implying the engine
  has been quiet since it started
