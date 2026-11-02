# Spec Delta

## MODIFIED Requirements

### Requirement: Choosing a node

Selection SHALL query every candidate node concurrently, as `spinloop fleet
status` does, and SHALL prefer a node that is already running what is wanted: a
node whose state is `running` and whose served model matches the Spinloop's
`MODEL` (or its `ALIAS`, against the name the node reports serving). A Spinloop
that names no model SHALL match any running node.

A node whose state is `running` but whose daemon reports the engine not ready is
not running what is wanted: the state turns running when the engine's process
exists, which is before the weights are fetched and loaded, and during that
window nothing is listening on the engine's port. Such a node SHALL NOT be
selected, and a failure that names it SHALL mark it not ready, so a refusal does
not read as if the node were serving the model it was asked for. A node whose
daemon reports no readiness at all — an older build, or a runner with no
health-check convention — SHALL NOT be disqualified: the absence of a reading is
not evidence of not-readiness.

Matching nodes SHALL be ranked by the activity preference in force (see
"Preferring an idle or an active node"). Ties SHALL be broken by fleet-file
order, so the same fleet in the same state chooses the same node.

A node that does not answer — unreachable, unauthorized, or a configuration
error — SHALL be skipped rather than aborting the selection, exactly as it is a
row rather than a failure in `spinloop fleet status`.

`--node <name>` — or its `-n <name>` short form — SHALL pin the selection to
one node, skipping the search. It is the one name a launch gives for the node
its agent talks to: with a fleet in force the name is a node of that fleet;
with no fleet in force the same flag instead names a registered environment,
which is a fleet of one (see the cloud-environment capabilities), and this
selection never runs for it. An unknown name SHALL fail naming the known nodes,
and a pinned node that cannot be reached SHALL fail rather than falling back to
another node — a pin is an instruction, not a preference. A pinned node that is running the wanted model
but has not answered yet SHALL fail saying it is still starting it — it may be
fetching or loading weights — and naming the command for the node's log, rather
than saying that nothing serves the model or restarting the node: this node is
about to serve it.

A running engine SHALL NEVER be stopped or restarted to make room, including a
pinned one: another person may be using it. A node running a different model is
therefore not a candidate, and pinning one SHALL fail saying what it is serving.

#### Scenario: The preference decides between matching nodes

- **WHEN** two nodes are running the wanted model and one reports a longer time
  since it last did work
- **THEN** the one the activity preference favours is chosen, and the same fleet
  in the same state chooses the same node every time

#### Scenario: An unreachable node is skipped

- **WHEN** one node in the fleet cannot be reached and another is running the
  wanted model
- **THEN** the reachable node is chosen and the launch proceeds

#### Scenario: A pinned node is used as given

- **WHEN** the user runs `spinloop harness --node gpu-box` and that node is
  running the wanted model
- **THEN** `gpu-box` is chosen without regard to what the other nodes are doing

#### Scenario: A pinned node that cannot be reached fails

- **WHEN** the user pins a node whose daemon is unreachable
- **THEN** the command fails naming that node, and no other node is selected

#### Scenario: A not-ready engine is not a match

- **WHEN** the only node running the wanted model reports its engine not ready,
  and no other node is running it
- **THEN** the selection reports that nothing is serving the model, and the
  failure names the node marked not ready

#### Scenario: A missing readiness reading still routes

- **WHEN** a node is running the wanted model and its daemon reports no
  readiness at all
- **THEN** the node is chosen, as if its engine had answered

#### Scenario: A pinned node that is still starting names its state

- **WHEN** the user pins a node whose engine is running the wanted model but has
  not answered yet
- **THEN** the command fails saying the node is still starting the model, that
  it may be fetching or loading weights, and names the command for the node's
  log, without restarting the node

#### Scenario: A busy node is left alone

- **WHEN** every reachable node is running a model other than the one wanted
- **THEN** no running engine is stopped, and selection falls through to waking
  an idle node

#### Scenario: Pinning a node serving something else fails

- **WHEN** the user pins a node that is running a different model
- **THEN** the command fails saying what that node is serving, and the engine is
  untouched

## ADDED Requirements

### Requirement: A launch routed through a fleet or pinned to a node
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
holding a `fleet.yaml` or not — unless `--node` names a target alongside a
fleet in force, which routes: it is served by the fleet's gateway where one is
declared, and otherwise fails saying the Spinloop is what decides which node
can serve. Where a Spinloop is worn and neither a flag nor a directory
`./fleet.yaml` is in force, the launch SHALL behave exactly as it does without
routing.

Routing SHALL choose one node and give the launched agent that
node's engine as its OpenAI-compatible endpoint: the chosen base URL SHALL be
written as the applied provider's base URL, in the same place an environment's
address is written, and SHALL also be placed in the launched agent's
environment as `OPENAI_BASE_URL`.

A variable already set in spinloop's environment SHALL win, as it does on the
cloud path — routing fills what is unset, it does not override an explicit
choice.

A Spinloop that pins a `BASEURL` SHALL NOT be routed: the pinned address wins
and spinloop SHALL say it is not routing through the fleet, rather than
silently selecting a node whose address it then discards.

A launch given `--node <name>` — or its `-n <name>` short form — alongside a
fleet, whether the directory's `fleet.yaml` or a `--fleet` flag, SHALL pin the
selection to that fleet's node of that name, as above: naming a node and the
fleet it belongs to is one answer, not a conflict. A `--node` whose name is not
a node of the fleet in force SHALL fail naming that fleet's known nodes rather
than resolving the name as a registered environment — with a fleet in force the
name is a fleet node's, always. Where no fleet is in force the same flag names
a registered environment instead.

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

#### Scenario: --node with a fleet pins that fleet's node

- **WHEN** the user runs `spinloop harness --node dev-2` in a directory holding
  a `fleet.yaml` that has no node named `dev-2`
- **THEN** the launch fails naming `dev-2` and the fleet's known nodes, and does
  not fall back to a registered environment named `dev-2`

#### Scenario: --node with no fleet names a registered environment

- **WHEN** the user runs `spinloop harness -O --node dev-2` in a directory
  holding a `Spinloop` and no `fleet.yaml`, passing no `--fleet`
- **THEN** the launch is applied against the registered environment `dev-2`, as
  the same launch formerly given `--env dev-2` was

#### Scenario: The choice is announced

- **WHEN** a fleet-routed launch selects a node
- **THEN** the node's name, the resolved endpoint, and why it was chosen are
  written to stderr before the harness is launched

## REMOVED Requirements

### Requirement: A launch routed through a fleet
**Reason**: The requirement's scenarios name the target flag in their titles; `--env`/`-e` is renamed `--node`/`-n`, and a MODIFIED cannot retitle scenarios. 
**Migration**: The requirement "A launch routed through a fleet or pinned to a node" states the same contract with the renamed flag.
