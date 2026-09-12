## ADDED Requirements

### Requirement: Status reports the name the model is served under

Status SHALL report, beside the model id it already reports, the name the
engine serves the model under — the deploy config's served name — when the
stored config names one. A router that sees only status needs the name a
request will carry: a client that started the engine under an alias addresses
it by that alias, and matching a request only against the model id would not
recognise the node. The served name SHALL be omitted when the stored config
names none, and the model id SHALL be reported on the same terms it is today
whatever the served name is.

#### Scenario: An aliased engine reports both names

- **WHEN** an engine was started from a config that names both a model and a
  served name, and a status request is made
- **THEN** the response reports the model id and the served name

#### Scenario: No served name, no field

- **WHEN** an engine was started from a config that names no served name, and a
  status request is made
- **THEN** the response reports the model id and carries no served name
