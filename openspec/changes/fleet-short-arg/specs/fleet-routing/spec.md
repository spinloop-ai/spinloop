## MODIFIED Requirements

### Requirement: A fleet-routed launch

`spinloop harness` SHALL route through a fleet when the Spinloop it wears names one
with a `FLEET` instruction, or when `--fleet <path>` — or its `-f <path>`
short form — is given; the flag SHALL
override the instruction, and a launch with neither SHALL behave exactly as it
does today. Routing SHALL choose one node and give the launched agent that
node's engine as its OpenAI-compatible endpoint: the chosen base URL SHALL be
written as the applied provider's base URL, in the same place a `REMOTE`
endpoint's address is written, and SHALL also be placed in the launched agent's
environment as `OPENAI_BASE_URL`.

A variable already set in spinloop's environment SHALL win, as it does on the
remote path — routing fills what is unset, it does not override an explicit
choice.

A Spinloop that pins a `BASEURL` SHALL NOT be routed: the pinned address wins and
spinloop SHALL say it is not routing through the fleet, rather than silently
selecting a node whose address it then discards.

The chosen node and the reason it was chosen SHALL be reported on stderr before
the agent launches, so a launch that lands somewhere unexpected says so at the
time rather than at the first request.

#### Scenario: A running node becomes the agent's endpoint

- **WHEN** the user runs `spinloop harness` with a Spinloop naming a `FLEET`, and a
  node in that fleet is running the model the Spinloop names
- **THEN** the launched agent's environment carries `OPENAI_BASE_URL` pointing
  at that node's engine, and the applied provider's base URL is the same address

#### Scenario: The flag overrides the instruction

- **WHEN** the user runs `spinloop harness --fleet=./cluster.yaml` with a Spinloop
  whose `FLEET` names a different file
- **THEN** the nodes in `./cluster.yaml` are the candidates

#### Scenario: The short form overrides the instruction

- **WHEN** the user runs `spinloop harness -f ./cluster.yaml` with a Spinloop
  whose `FLEET` names a different file
- **THEN** the nodes in `./cluster.yaml` are the candidates

#### Scenario: A Spinloop with no FLEET is unaffected

- **WHEN** the user runs `spinloop harness` with a Spinloop naming no `FLEET` and
  passes no `--fleet`
- **THEN** no fleet file is read, no node is contacted, and the launch behaves
  as it did before

#### Scenario: A pinned BASEURL is not routed

- **WHEN** a Spinloop names both a `FLEET` and a `BASEURL`
- **THEN** the `BASEURL` is used, no node is selected, and spinloop reports that
  it is not routing through the fleet

#### Scenario: An exported base URL wins

- **WHEN** `OPENAI_BASE_URL` is already set in the user's environment and a
  fleet-routed launch runs
- **THEN** the existing value reaches the agent unchanged

#### Scenario: The choice is announced

- **WHEN** a fleet-routed launch selects a node
- **THEN** the node's name, the resolved endpoint, and why it was chosen are
  written to stderr before the harness is launched
