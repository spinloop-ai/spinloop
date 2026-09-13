# Delta: fleet-routing

## MODIFIED Requirements

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
written as the applied provider's base URL, in the same place an environment's
address is written, and SHALL also be placed in the launched agent's
environment as `OPENAI_BASE_URL`.

A variable already set in spinloop's environment SHALL win, as it does on the
remote path — routing fills what is unset, it does not override an explicit
choice.

A Spinloop that pins a `BASEURL` SHALL NOT be routed: the pinned address wins
and spinloop SHALL say it is not routing through the fleet, rather than
silently selecting a node whose address it then discards.

A launch given `--env <name>` — or its `-e <name>` short form — alongside a
fleet, whether the directory's `fleet.yaml` or a `--fleet` flag, SHALL fail
naming both: each is an answer to where the model is served from — one names a
specific environment, the other a set of nodes to choose between — and a launch
stating both is a mistake rather than a precedence to resolve.

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

- **WHEN** the user runs `spinloop harness -O -f ./cluster.yaml` in a
  directory holding a `Spinloop` and a `fleet.yaml`
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

#### Scenario: --env and a fleet conflict

- **WHEN** the user runs `spinloop harness --env dev-2` in a directory holding
  a `fleet.yaml`, or with a `--fleet` flag
- **THEN** the launch fails naming both the `--env` flag and the fleet

#### Scenario: The choice is announced

- **WHEN** a fleet-routed launch selects a node
- **THEN** the node's name, the resolved endpoint, and why it was chosen are
  written to stderr before the harness is launched
