## ADDED Requirements

### Requirement: The stats reply carries the retention deadline

The stats Lambda's reply SHALL carry the instance's retention deadline — the
time until which the idle sweep leaves the instance alone, parsed from the
instance's Retain-Until tag, which the Lambda already reads — when that tag is
a time in the future, and SHALL omit it when the tag is absent or a time that
has passed. The deadline is a fact the control plane holds about the instance,
not one the on-instance daemon reports: it SHALL be present for a stopped
instance the sweep would otherwise terminate, and absent for an environment
with no instance at all.

A control plane that predates the field SHALL simply omit it, with no error:
every reader of the reply treats an absent deadline as "no active retention"
rather than as a failure.

The client SHALL map the deadline onto the shared stats shape every fleet and
remote stats surface reads from, so a dashboard panel, a one-shot fleet report,
and a one-shot remote report cannot word the same read differently.

The deadline SHALL be reported beside the report's other time facts — the
last-active figure — in every format the command supports, and omitted in
every format when the read carries none, following the same omission rule the
last-active figure uses for a figure it does not have.

#### Scenario: A retained instance's stats carry the deadline

- **WHEN** the user reads the stats of an environment whose instance carries a
  Retain-Until tag at a time in the future
- **THEN** the reply carries that deadline

#### Scenario: A passed tag carries no deadline

- **WHEN** the user reads the stats of an instance whose Retain-Until tag is a
  time that has passed
- **THEN** the reply carries no deadline: a passed tag holds no active
  retention

#### Scenario: An untagged or instance-less environment carries no deadline

- **WHEN** the user reads the stats of an instance with no Retain-Until tag,
  or of an environment with no instance
- **THEN** the reply carries no deadline

#### Scenario: An older control plane is read without error

- **WHEN** the user reads the stats of an environment whose control plane
  predates the field
- **THEN** the read succeeds, the deadline is simply absent, and the rest of
  the report renders as before

#### Scenario: Every format carries the deadline

- **WHEN** the user reads the stats of a retained environment with `bar`,
  `table`, or `json` output
- **THEN** each output carries the deadline in its own idiom, beside the
  last-active figure, and each omits it when the read carries none
