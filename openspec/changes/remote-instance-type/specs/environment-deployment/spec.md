## ADDED Requirements

### Requirement: Deploy accepts an optional instance type

`spinloop remote deploy` SHALL accept an optional `--instance-type` flag naming
the EC2 instance type the environment's instances launch as. When the flag is
given, deploy SHALL record that type in the environment's stored deploy config
so the environment's next fresh launch uses it; when it is absent, deploy SHALL
record no type and the environment launches as the control plane's default. An
empty or whitespace-only value SHALL be treated as if the flag were not given.
A value that is not shaped like an EC2 instance type SHALL be refused before
anything is sent, naming the value.

The instance type is a property of the environment's deployment, recorded in
the stored deploy config the way the spinloop version pin is — not a property
of a single start, and never derived from the Spinloop.

#### Scenario: A type is recorded in the deploy config

- **WHEN** `spinloop remote deploy` runs with `--instance-type g6e.2xlarge`
- **THEN** the environment's stored deploy config carries that type, and the
  environment's next fresh launch uses it

#### Scenario: No type leaves the launch on its default

- **WHEN** `spinloop remote deploy` runs without `--instance-type`
- **THEN** the stored deploy config carries no instance type, and the
  environment's launches use the control plane's default type

#### Scenario: An empty type value is ignored

- **WHEN** `spinloop remote deploy` is given an `--instance-type` whose value
  is empty or whitespace only
- **THEN** it is treated as if no type were given

#### Scenario: A malformed type is refused before sending

- **WHEN** `spinloop remote deploy` is given an `--instance-type` that is not
  shaped like an EC2 instance type
- **THEN** the command fails, naming the value, and nothing is sent to the
  control plane

### Requirement: The deploy plan shows the resolved instance type

The plan `spinloop remote deploy` prints — including under `--dry-run`, before
any AWS work or send — SHALL state the instance type the environment will
launch as: the type named by `--instance-type` when one is given, otherwise a
statement that the environment launches as the control plane's default. It
SHALL appear alongside the runner and model the plan already prints.

#### Scenario: A typed deploy prints the type

- **WHEN** `spinloop remote deploy --dry-run` runs with `--instance-type
  g6e.2xlarge`
- **THEN** the printed plan names `g6e.2xlarge` as the instance type the
  environment will launch as

#### Scenario: An untyped deploy prints the default

- **WHEN** `spinloop remote deploy --dry-run` runs without `--instance-type`
- **THEN** the printed plan says the environment launches as the control
  plane's default instance type
