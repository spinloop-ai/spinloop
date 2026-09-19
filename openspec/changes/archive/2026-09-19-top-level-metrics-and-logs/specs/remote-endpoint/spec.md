## MODIFIED Requirements

### Requirement: Remote command group

The system SHALL provide a `remote` command group with the subcommands
`bootstrap`, `bake`, `auth`, `start`, `stop`, `restart`, `deploy`,
`ls`, and `keep`. `start`, `stop`, `restart` and `deploy` each take an
optional Spinloop path:
`start` SHALL boot the endpoint and block until it is serving, then perform a
quick TCP probe of the inference endpoint — if the probe fails, a warning is
printed to stderr explaining the network mismatch (see the Remote Start Probe
specification) — and finally print the base URL and API key as shell exports;
`start` SHALL also accept a `--keep DURATION` flag that sets the instance
retention deadline to `now + DURATION`, preventing the idle sweep from
terminating it before that time (see the Remote Keep specification);
`stop` SHALL stop it immediately rather than waiting for its idle timer;
`restart` SHALL stop the endpoint in the manner of a pause — without
terminating it, so its boot disk, its weights and its stable address are
preserved — and SHALL immediately start it again, blocking until it is serving
and reporting progress as `start` does (see the Reporting a start in progress
specification); `restart` SHALL accept a `--force` flag with a `-F` short form
that, when set, performs the stop without first asking the engine to shut down
(see the Endpoint Lifecycle specification for forced stops);
`keep` SHALL set the `Retain-Until` tag on the environment's instance for the
given duration, without starting or stopping the instance (see the Remote Keep
specification); `deploy` SHALL set what the endpoint serves. `ls` SHALL list the registered remote environments
(see the Remote Environments specification). `bootstrap` SHALL stand up the
account-level AWS control plane (once per account) by obtaining and driving the
CDK project, and takes its own flags rather than a Spinloop path (see the
Endpoint Provisioning specification). `bake` SHALL start an AMI bake for each
runner named, and takes runner names rather than a Spinloop path (see the
Endpoint Provisioning specification). `auth` SHALL store, report, and clear the
long-lived control-plane credential, and takes its own flags rather than a
Spinloop path (see the Remote Auth specification). An unrecognised subcommand
SHALL fail naming the accepted ones.

#### Scenario: Starting the endpoint

- **WHEN** the user runs `spinloop remote start` and the endpoint reports ready
- **THEN** the base URL and API key are printed as `export` lines

#### Scenario: Starting warns when the network is not admitted

- **WHEN** the user runs `spinloop remote start` and the endpoint reports ready
  but the TCP probe to the inference port fails
- **THEN** a warning is printed to stderr with a remediation command, and the
  command still exits 0

#### Scenario: Starting with a keep flag

- **WHEN** the user runs `spinloop remote start --keep 4h` and the endpoint reports ready
- **THEN** the base URL and API key are printed as `export` lines, and the
  instance retention deadline is set to 4 hours from now

#### Scenario: Waiting through a cold start

- **WHEN** the endpoint reports that it is still starting
- **THEN** the command waits and retries until it is ready or the timeout
  passes, rather than failing on the first attempt

#### Scenario: Restarting the endpoint

- **WHEN** the user runs `spinloop remote restart` for a running environment and
  the endpoint reports ready again
- **THEN** the instance was stopped and re-woken without being terminated, the
  command blocked until the model was serving again, and the environment's
  address is the one its configuration records

#### Scenario: Forcing a restart skips the engine stop

- **WHEN** the user runs `spinloop remote restart --force` (or `-F`)
- **THEN** the instance is stopped without the engine being asked to shut down
  first, and the command then blocks until the model is serving again

#### Scenario: Restarting a stopped endpoint starts it

- **WHEN** the user runs `spinloop remote restart` for an environment whose instance is already stopped
- **THEN** the instance is re-woken rather than replaced, and the command blocks
  until the model is serving again, as with a plain start

#### Scenario: A failed re-wake says how to recover

- **WHEN** the stop half of a restart has taken effect but the wake fails
- **THEN** the command fails saying the instance is stopped and that
  `spinloop remote start` will bring it back

#### Scenario: Listing environments

- **WHEN** the user runs `spinloop remote ls`
- **THEN** the registered environments are listed rather than any endpoint being
  contacted

#### Scenario: Setting a keep deadline

- **WHEN** the user runs `spinloop remote keep 2h`
- **THEN** the instance retention tag is set and the deadline is reported

#### Scenario: Metrics reports instance figures

- **WHEN** the user runs `spinloop remote metrics` with a running instance
- **THEN** token counts, resource usage, and GPU information are displayed

#### Scenario: Bootstrap is a recognised subcommand

- **WHEN** the user runs `spinloop remote bootstrap`
- **THEN** the command is dispatched to the provisioning flow rather than
  reported as unknown

#### Scenario: Bake is a recognised subcommand

- **WHEN** the user runs `spinloop remote bake llamacpp`
- **THEN** the command is dispatched to the bake flow rather than
  reported as unknown

#### Scenario: Auth is a recognised subcommand

- **WHEN** the user runs `spinloop remote auth`
- **THEN** the command is dispatched to the credential store, report, and clear
  flow rather than reported as unknown

#### Scenario: Unknown subcommand

- **WHEN** the user runs `spinloop remote frobnicate`
- **THEN** the command fails listing the accepted subcommands, which include
  `bootstrap`, `bake`, `metrics`, and `keep`

#### Scenario: The read verbs are not in the group

- **WHEN** the operator runs `spinloop remote status`, `spinloop remote
  metrics` or `spinloop remote logs`
- **THEN** each fails naming the top-level verb that replaced it, and the
  group's help lists none of them

## REMOVED Requirements

### Requirement: Status reports when the endpoint last did work

**Reason**: `spinloop remote status` is removed — one command reports an
engine's state, whether it is named as an environment or as a fleet node.
The behaviour is unchanged and is restated below under a name that does not
carry the removed command's spelling.

**Migration**: `spinloop status --env <name>`.

### Requirement: Status degrades when activity cannot be read

**Reason**: `spinloop remote status` is removed — one command reports an
engine's state, whether it is named as an environment or as a fleet node.
The behaviour is unchanged and is restated below under a name that does not
carry the removed command's spelling.

**Migration**: `spinloop status --env <name>`.

## ADDED Requirements

### Requirement: An environment reports when it last did work

`spinloop status --env <name>` SHALL report how long it has been since the endpoint's
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

- **WHEN** the user runs `spinloop status --env <name>` against a running endpoint
  whose engine has served work
- **THEN** the output reports how long ago that work happened, labelled "last
  active", beside the state and health lines

#### Scenario: Status stays a read

- **WHEN** the user runs `spinloop status --env <name>`
- **THEN** nothing is started, stopped or probed in order to obtain the
  last-active figure

### Requirement: An environment's status degrades when activity cannot be read

`spinloop status --env <name>` SHALL omit the last-active figure rather than fail,
report zero, or imply inactivity, whenever the figure cannot be obtained. That
covers an endpoint whose engine has not yet done any work, a daemon that
cannot be reached or answers unrecognisably, and an instance that is not
running — reaching the daemon needs a running box, so a stopped or undeployed
environment has nothing to report about its engine.

A failure to read the activity SHALL NOT affect the rest of the report: the
state and health lines SHALL be exactly what they are today, and the command
SHALL still succeed.

#### Scenario: A stopped instance reports no activity figure

- **WHEN** the user runs `spinloop status --env <name>` and the instance is stopped or
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
