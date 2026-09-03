## MODIFIED Requirements

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
and a node now serving what was wanted SHALL be used. Losing that race is
another route to the same place, not an error.

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
- **THEN** the launch uses that node rather than failing

#### Scenario: A node that never comes up

- **WHEN** a woken node does not report running within the timeout
- **THEN** the command fails naming the node, and the engine it started is left
  running rather than stopped

#### Scenario: Waking is refused

- **WHEN** `--no-wake` is passed and no node is serving the wanted model
- **THEN** the command fails, listing the nodes with their states and naming the
  `spinloop fleet start` command that would start one, and nothing is started
