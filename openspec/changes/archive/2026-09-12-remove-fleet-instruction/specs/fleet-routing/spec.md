## ADDED Requirements

### Requirement: A launch routed through a fleet

`spinloop harness` SHALL route a worn Spinloop through a fleet when
`--fleet <path>` — or its `-f <path>` short form — is given, and, when the
worn Spinloop was not named explicitly, through a `./fleet.yaml` in the
working directory. A worn Spinloop is named explicitly when the user gives its
path — a leading positional argument or a `--spinloop`/`-O` value — or when
`SPINLOOP_ALIAS` is set; a valueless `--spinloop` wears the default Spinloop
and is not named. An explicitly named Spinloop SHALL NOT pick up a fleet file
from the working directory on its own: the user pointed at that file
deliberately, and routing happens only when `--fleet` is also given. A launch
that wears no Spinloop applies nothing and routes nothing, in a directory
holding a `fleet.yaml` or not. Where a Spinloop is worn and neither a flag nor
a directory `./fleet.yaml` is in force, the launch SHALL behave exactly as it
does without routing.

Routing SHALL choose one node and give the launched agent that
node's engine as its OpenAI-compatible endpoint: the chosen base URL SHALL be
written as the applied provider's base URL, in the same place a `REMOTE`
endpoint's address is written, and SHALL also be placed in the launched agent's
environment as `OPENAI_BASE_URL`.

A variable already set in spinloop's environment SHALL win, as it does on the
remote path — routing fills what is unset, it does not override an explicit
choice.

A Spinloop that pins a `BASEURL` SHALL NOT be routed: the pinned address wins
and spinloop SHALL say it is not routing through the fleet, rather than
silently selecting a node whose address it then discards.

The chosen node and the reason it was chosen SHALL be reported on stderr before
the agent launches, so a launch that lands somewhere unexpected says so at the
time rather than at the first request.

#### Scenario: A running node becomes the agent's endpoint

- **WHEN** the user runs `spinloop harness -O` in a directory holding a
  `Spinloop` and a `fleet.yaml`, and a node in that fleet is running the model
  the Spinloop names
- **THEN** the launched agent's environment carries `OPENAI_BASE_URL` pointing
  at that node's engine, and the applied provider's base URL is the same
  address

#### Scenario: The flag overrides the directory's fleet file

- **WHEN** the user runs `spinloop harness -O --fleet=./cluster.yaml` in a
  directory holding a `Spinloop` and a `fleet.yaml`
- **THEN** the nodes in `./cluster.yaml` are the candidates, not the ones in
  the directory's `fleet.yaml`

#### Scenario: The short form overrides the directory's fleet file

- **WHEN** the user runs `spinloop harness -O -f ./cluster.yaml` in a directory
  holding a `Spinloop` and a `fleet.yaml`
- **THEN** the nodes in `./cluster.yaml` are the candidates, not the ones in
  the directory's `fleet.yaml`

#### Scenario: No fleet file in force leaves the launch local

- **WHEN** the user runs `spinloop harness -O` in a directory holding a
  `Spinloop` and no `fleet.yaml`, passing no `--fleet`
- **THEN** no fleet file is read, no node is contacted, and the launch behaves
  as it did before

#### Scenario: An explicitly named Spinloop does not route

- **WHEN** the user runs `spinloop harness ./elsewhere/Spinloop` in a directory
  holding a `fleet.yaml`, and passes no `--fleet`
- **THEN** no fleet file is read, no node is contacted, and the launch applies
  the Spinloop locally

#### Scenario: The flag routes an explicitly named Spinloop

- **WHEN** the user runs `spinloop harness ./elsewhere/Spinloop --fleet
  ./cluster.yaml`
- **THEN** the nodes in `./cluster.yaml` are the candidates

#### Scenario: An aliased Spinloop does not route

- **WHEN** `SPINLOOP_ALIAS` is set and the user runs `spinloop harness -O` in
  a directory holding a `fleet.yaml`, passing no `--fleet`
- **THEN** no fleet file is read, and the launch applies the aliased Spinloop
  locally

#### Scenario: A pinned BASEURL is not routed

- **WHEN** a `Spinloop` beside a `fleet.yaml` names a `BASEURL`, and the user
  runs `spinloop harness -O`
- **THEN** the `BASEURL` is used, no node is selected, and spinloop reports
  that it is not routing through the fleet

#### Scenario: An exported base URL wins

- **WHEN** `OPENAI_BASE_URL` is already set in the user's environment and a
  fleet-routed launch runs
- **THEN** the existing value reaches the agent unchanged

#### Scenario: The choice is announced

- **WHEN** a fleet-routed launch selects a node
- **THEN** the node's name, the resolved endpoint, and why it was chosen are
  written to stderr before the harness is launched

## MODIFIED Requirements

### Requirement: A fleet file naming a gateway routes the launch at it

A launch whose effective fleet file — one given by `--fleet`/`-f`, or the
`./fleet.yaml` in the working directory where the worn Spinloop was not named
explicitly — declares a `gateway` section SHALL route at that
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

## REMOVED Requirements

### Requirement: A fleet-routed launch

**Reason**: Its fleet sources were the Spinloop's `FLEET` and the flag; the
`FLEET` is removed, and the remaining sources are the flag and a `./fleet.yaml`
in the working directory for a Spinloop the user did not name. The scenarios
tied to the `FLEET` cannot stand under the new rules, so the requirement is
restated wholesale.

**Migration**: See `A launch routed through a fleet` in this capability, which
carries the same behaviour with the new discovery rules.
