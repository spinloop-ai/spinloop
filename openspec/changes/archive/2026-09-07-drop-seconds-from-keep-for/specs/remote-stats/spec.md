## MODIFIED Requirements

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

The deadline SHALL be reported on the report's active-figure line — rendered by
the client as a relative remaining time, e.g. `keep for 2h`, not the absolute
timestamp — in every format the command supports, and omitted in every format
when the read carries none, following the same omission rule the active figure
uses for a figure it does not have.

The relative remaining time SHALL be worded in hours and minutes only, with
any zero unit dropped — `keep for 2h`, `keep for 1h 30m`, `keep for 24m` — and
SHALL NOT carry a seconds component: a keep is set in minutes or hours, and in
a panel that re-renders, a seconds figure changes on every refresh without
changing what the operator can do about it. A remaining time of less than a
minute SHALL still render — as `keep for 1m` — so the figure is present for
any future deadline and absent only once the deadline has passed.

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
- **THEN** each output carries the deadline in its own idiom on the active
   figure's line, and each omits it when the read carries none

#### Scenario: The relative time carries no seconds

- **WHEN** the user reads the stats of a retained environment whose deadline
  is two hours, five minutes, and thirty seconds away
- **THEN** the report's active-figure line renders `keep for 2h 5m`

#### Scenario: A whole-hour or whole-minute deadline drops zero units

- **WHEN** the user reads the stats of a retained environment whose deadline
  is exactly two hours away, and later of one that is exactly twenty-four
  minutes away
- **THEN** the report's active-figure line renders `keep for 2h`, and later
  `keep for 24m`

#### Scenario: A sub-minute keep still renders

- **WHEN** the user reads the stats of a retained environment whose deadline
  is forty-five seconds away
- **THEN** the report's active-figure line renders `keep for 1m`
