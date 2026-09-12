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

`--node <name>` SHALL pin the selection to one node, skipping the search. An
unknown name SHALL fail naming the known nodes, and a pinned node that cannot be
reached SHALL fail rather than falling back to another node — a pin is an
instruction, not a preference. A pinned node that is running the wanted model
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

### Requirement: Waking a node

When no running node is serving what is wanted, routing SHALL wake one: it SHALL
choose a node that is not running, push what the Spinloop asks for as that node's
deploy config, start it through the daemon's start endpoint, and wait before
launching the agent — not merely until the node reports `running`, which says
only that a process exists, but until its engine endpoint answers. A node whose
stored config already matches the wanted model SHALL be preferred, since it has
the weights.

The pushed config is the node-side counterpart of what `spinloop serve` would run
for that Spinloop, translated per engine. A node may be woken for an engine that
binds its model at launch — `llamacpp`, `vllm`, and `mtplx` — and a `MODEL` that
names a file on the node's own disk is a valid wake for it: the node has the
file, and only a destination that fetches its weights itself refuses a local
path.

A node that refuses the config — a runner or model it cannot serve — SHALL NOT
fail the launch while other candidates remain: the next candidate SHALL be
tried, and the refusals SHALL be reported when none succeeds.

Two clients may wake the same node at once. A start refused because an engine is
already running SHALL NOT fail the launch: the node's state SHALL be re-read,
and a node now serving what was wanted SHALL be used — and the launch SHALL
wait for that node's engine to answer before launching the agent, exactly as it
waits for a node it woken itself: the node that won the race may still be
loading weights, and the wait is bounded by the same timeout. Losing that race
is another route to the same place, not an error.

The wait SHALL be bounded by a timeout and SHALL report what it is waiting for,
because a cold node loads weights before it answers. Exceeding the timeout SHALL
fail naming the node and the endpoint that did not come up; the started engine
SHALL be left running rather than stopped, so a slow load is not thrown away.

`--no-wake` SHALL turn waking off: with no running node serving what is wanted
the command SHALL then fail, listing the nodes and their states and naming the
command that would start one.

#### Scenario: An idle node is woken and used

- **WHEN** a fleet-routed launch finds no node serving the wanted model and one
  node is idle and able to serve it
- **THEN** that node is given the Spinloop's model as its deploy config, started,
  and the agent launches against it once its engine answers

#### Scenario: A node is woken for a Mac-only engine

- **WHEN** a fleet-routed launch finds no node serving the wanted model, and an
  idle node's daemon can run MTPLX
- **THEN** that node is woken with a config that runs the wanted model under
  `mtplx serve`, and the agent launches against it once its engine answers

#### Scenario: A local model path wakes the node that has it

- **WHEN** the Spinloop's `MODEL` names a file on the woken node's disk
- **THEN** the wake carries that path as the model to load, rather than
  refusing it as a local file

#### Scenario: A Spinloop that pins a bind wakes a node bound to it

- **WHEN** the Spinloop names a `BASEURL` and a node is woken for it
- **THEN** the engine the node starts binds to the address the `BASEURL` names,
  exactly as `spinloop serve` would bind it, and the node reports that engine
  as reachable rather than on the engine's own default

#### Scenario: A started engine that is not yet loaded is waited for

- **WHEN** a woken node reports `running` while its engine is still loading
  weights and not yet answering
- **THEN** the launch waits for the engine to answer rather than launching the
  agent against an endpoint that refuses connections

#### Scenario: A node that cannot serve the model is passed over

- **WHEN** the first idle candidate rejects the pushed config as unservable and
  a second idle node accepts it
- **THEN** the second node is started and used

#### Scenario: No node can serve it

- **WHEN** every idle node rejects the config
- **THEN** the command fails, naming each node and the reason it refused

#### Scenario: Losing the race to another client

- **WHEN** a start is refused because another client woke the same node first,
  and that node is now serving the wanted model
- **THEN** the launch uses that node rather than failing, waiting for its
  engine to answer first if it is still loading

#### Scenario: A node that never comes up

- **WHEN** a woken node does not report running within the timeout
- **THEN** the command fails naming the node, and the engine it started is left
  running rather than stopped

#### Scenario: Waking is refused

- **WHEN** `--no-wake` is passed and no node is serving the wanted model
- **THEN** the command fails, listing the nodes with their states and naming the
  `spinloop fleet start` command that would start one, and nothing is started
