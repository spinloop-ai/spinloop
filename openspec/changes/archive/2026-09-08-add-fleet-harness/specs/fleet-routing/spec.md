## ADDED Requirements

### Requirement: A fleet file naming a gateway routes the launch at it

A launch whose effective fleet file — one given by `--fleet`/`-f`, or named
by the Spinloop's `FLEET` — declares a `gateway` section SHALL route at that
gateway the way a launch routes at an endpoint: the section's address SHALL be
written as the applied provider's base URL, with the OpenAI-compatible prefix
appended when it carries no path, and SHALL be placed in the launched
agent's environment as `OPENAI_BASE_URL`. No node SHALL be contacted and none
SHALL be woken: the gateway has already done the choosing.

The gateway's token SHALL be resolved from the variable the section names — or
from `OPENAI_API_KEY` where the section names none — through the client's
existing key chain, with a variable already set in spinloop's environment
winning. When the variable is set nowhere, the launch SHALL fail before the
harness config is written, naming the variable to set.

A Spinloop that pins a `BASEURL` SHALL NOT be routed at the section: the
pinned address wins, as it wins over an endpoint.

#### Scenario: A launch is pointed at the fleet's gateway

- **WHEN** the user runs a launch against a fleet file whose `gateway` section
  names an address, and the section's token variable is set
- **THEN** the launched agent's environment carries `OPENAI_BASE_URL` at the
  section's address with the OpenAI-compatible prefix, the applied provider's
  base URL is the same address, and no node is contacted

#### Scenario: A missing gateway token fails early

- **WHEN** a launch routes at a fleet file's `gateway` section and the
  variable the section names is set nowhere
- **THEN** the launch fails before the harness config is written, naming the
  variable to set

#### Scenario: A pinned BASEURL still wins over the section

- **WHEN** a Spinloop pins a `BASEURL` and its fleet file names a gateway
- **THEN** the `BASEURL` is used, the gateway is not, and the launch says it
  is not routing

### Requirement: fleet route answers a fleet file naming a gateway

`spinloop fleet route` on a Spinloop whose fleet file declares a `gateway`
section SHALL report that the gateway has already chosen: it SHALL name the
address the launch will be given, SHALL NOT query any node, and SHALL NOT wake
one.

#### Scenario: A route against a file naming a gateway names it

- **WHEN** the user runs `spinloop fleet route` on a Spinloop whose fleet file
  declares a `gateway` section
- **THEN** the output names the section's address as the one the launch will
  use, no node is queried, and nothing is started
