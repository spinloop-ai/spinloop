## MODIFIED Requirements

### Requirement: A fleet file MAY name a gateway

A fleet file MAY declare a top-level `gateway` section, beside `wake:` and
`prefer:`, naming the address the fleet is served under by its gateway: a
`url`, optionally a `tokenEnv` naming the variable that holds the gateway's
token, and optionally a `name` labelling the gateway. The section is how a
machine that holds the fleet file learns how to point a harness at the fleet
without naming the address in a Spinloop.

The section's `url` SHALL carry a scheme, the way an endpoint value does, and
a `gateway` section without one SHALL be refused, naming the missing field. A
`tokenEnv` SHALL name a variable, and where the section names none, the token
SHALL be resolved under `OPENAI_API_KEY`. A `name`, when given, SHALL label
the gateway wherever a launch through it needs to name the gateway rather
than a model — see the fleet-client capability's harness-naming behavior;
where the section names none, the address's host SHALL stand in. A file
without a `gateway` section SHALL behave exactly as it does today.

#### Scenario: A file names its gateway

- **WHEN** a fleet file declares a `gateway` section naming a `url` and a
  `tokenEnv`, and the file is read
- **THEN** the section's address and token variable are available to the
  commands that route a launch at the fleet

#### Scenario: A section without an address is refused

- **WHEN** a fleet file declares a `gateway` section naming no `url`
- **THEN** the file is refused, naming the missing field

#### Scenario: No section, no change

- **WHEN** a fleet file declares no `gateway` section
- **THEN** nothing about the file's behaviour changes

#### Scenario: A section names its gateway

- **WHEN** a fleet file declares a `gateway` section naming a `name`, and the
  file is read
- **THEN** that name is available to the commands that label the gateway,
  ahead of any address-derived fallback

#### Scenario: A section with no name falls back to the address

- **WHEN** a fleet file declares a `gateway` section naming a `url` but no
  `name`, and the file is read
- **THEN** the address's host is available as the gateway's label
