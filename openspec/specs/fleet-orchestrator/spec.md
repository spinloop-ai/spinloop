# fleet-orchestrator Specification

## Purpose

Let a backlog of work run against a fleet at a pace the fleet can absorb: the
`spinloop orchestrator` command holds the work items, reads the fleet's
topology from the fleet's gateway, admits items while the fleet's declared
concurrency limits allow, and runs each admitted item as a one-shot agent
whose inference goes through that gateway.

## Requirements

### Requirement: The orchestrator command

`spinloop orchestrator` SHALL run as a long-running foreground process, the
way `spinloop gateway` and `spinloop serve` do: its lifecycle is the
process's, and it is a top-level command beside them.

It SHALL take the address of a fleet's gateway and the path of its work items
file. The gateway's address comes from an explicit flag, or — where no flag
is given — from the `gateway` section of a fleet file, the one an explicit
fleet flag names or the file in the working directory; the flag, where
given, wins. The items file is a flag with a default of `./work.yaml` in the
working directory. It SHALL authenticate to the gateway with a bearer token
resolved the way the fleet file's gateway section resolves one: a flag
naming the environment variable, defaulting to `OPENAI_API_KEY` — the
section's own variable where the gateway comes from the file and the flag
was not given — and, where the gateway comes from the file, the value read
from the process environment first, then the `.env` beside the fleet file.

The fleet file, where the command reads it, is read for the gateway's
address and the token's variable and nothing else: no node fact in it
reaches the run, and the gateway is the run's only view of the fleet. Where
neither a flag nor a readable fleet file's section names a gateway, the
command SHALL fail before it works an item, naming the flag and the file. A
gateway it cannot reach, or will not authenticate it to, SHALL stop the
command with a message naming the gateway.

The command SHALL take a `--create-item-dirs` flag, defaulting to false:
where it is set, the orchestrator creates an item's missing directory before
launching its agent; where it is not, a missing directory fails the item.

#### Scenario: A gateway and an items file drive the command

- **WHEN** the operator runs the command naming a reachable gateway and an
  items file, with the gateway's token in the environment
- **THEN** it reads the fleet's topology from the gateway and works the
  items file's backlog

#### Scenario: A fleet file names the gateway

- **WHEN** the operator runs the command naming no gateway, and the fleet
  file — the one the fleet flag names, or the file in the working
  directory — names a gateway in its section
- **THEN** the command works against that gateway, authenticating under the
  section's token variable where the token flag was not given, the token's
  value read from the environment first, then the `.env` beside the file

#### Scenario: No gateway anywhere stops the command

- **WHEN** the operator names no gateway, and no readable fleet file — or
  none whose section names a gateway — is to hand
- **THEN** the command fails before it works an item, naming the flag and
  the file

#### Scenario: An unreachable gateway stops the command

- **WHEN** the operator runs the command naming a gateway that does not
  answer, or that refuses its token
- **THEN** the command fails, naming the gateway and what went wrong, and
  works no item

### Requirement: Work items

A work items file SHALL declare a list of items. Each item SHALL carry a
unique `id`, the instructions its agent is given, and the directory the agent
works in. An item MAY carry tags — a list of `key=value` pairs, the way node
tags are named — describing the nodes it can run on, and a numeric priority,
higher first, defaulting to last among equals by file order.

An item's id SHALL be unique within the file; a duplicate SHALL be an error
naming the collision. An item whose instructions or working directory is
missing SHALL be an error naming the item. An item that names no tags SHALL
match any node. A file the orchestrator cannot parse SHALL stop the command
naming the file, working no item.

#### Scenario: A minimal item

- **WHEN** an items file lists an item with an id, instructions, and a
  working directory and nothing else
- **THEN** the item enters the backlog, matching any node, at the lowest
  priority

#### Scenario: Duplicate ids are rejected

- **WHEN** an items file lists two items with the same id
- **THEN** the command fails, naming the duplicated id, and works no item

#### Scenario: A new item in the file enters the backlog

- **WHEN** the items file gains an item the orchestrator has not seen, while
  it runs
- **THEN** the item enters the backlog and is eligible for admission on the
  next pass

### Requirement: Reading the fleet's topology

The orchestrator SHALL learn the fleet's nodes, tags, states, serving facts,
and concurrency limits only from the gateway's topology: the run itself SHALL
NOT hold a fleet file, a node's token, or an engine key, and it SHALL NOT
contact a node's control API directly. The command may read a fleet file to
find the gateway, and nothing else from it reaches the run. It SHALL re-read
the topology as the fleet changes, so a node that starts, stops, or changes
what it serves is seen without the orchestrator being restarted.

#### Scenario: The gateway is the only fleet view

- **WHEN** the orchestrator runs
- **THEN** every fact it uses about the fleet — a node's tags, state, model,
  and the limits — came from the gateway's topology

#### Scenario: A changing fleet is seen

- **WHEN** a node the orchestrator last saw stopped comes back serving,
  without the orchestrator being restarted
- **THEN** the orchestrator's next pass sees it serving, and items that
  match it become eligible

### Requirement: Matching an item to a node

An item SHALL match a node only where every tag the item carries is a tag the
node carries: an item carrying `gpu=a100` and `os=linux` matches only a node
carrying both. Among matching nodes, a node that is running and whose engine
has answered SHALL be preferred over a node that would have to be started,
and matching nodes SHALL be ranked the way the fleet file's preference
ranks them. A node that is not running SHALL be eligible only where the
fleet's wake policy allows starting it, and it SHALL be offered the model a
request would start it with. An item no node matches SHALL wait in the
backlog, not fail: a node may come up, or be tagged, later.

#### Scenario: An item takes a node carrying all its tags

- **WHEN** an item carries two tags and two nodes each carry one of them,
  while a third node carries both
- **THEN** the item matches only the third node

#### Scenario: A running node is offered before a stopped one

- **WHEN** an item matches one running node and one stopped node, and the
  fleet wakes
- **THEN** the running node is offered first

#### Scenario: A stopped node is offered only where the fleet wakes

- **WHEN** an item matches only a stopped node, and the fleet's wake policy
  refuses to start
- **THEN** the item waits in the backlog

#### Scenario: An item nothing matches waits

- **WHEN** an item's tags match no node
- **THEN** the item stays in the backlog, and is tried again as the fleet
  changes

### Requirement: Admitting items under the limits

The orchestrator SHALL admit an item to run only while every limit the item
counts against has room: the fleet's total, where declared, against the
items it has in flight, and each per-tag limit, against the items it has in
flight that carry that tag. An item counts against every tag it carries, and
against the total. Where the fleet declares no limits, the orchestrator
admits as many items as nodes will take. Admitted items run no further than
the limits allow at once: when an item finishes, another may be admitted on
the next pass.

#### Scenario: The total bounds the in-flight

- **WHEN** the fleet's total is three and three items are in flight
- **THEN** no further item is admitted, whatever its tags

#### Scenario: A tag limit bounds its own items

- **WHEN** the limit on `gpu=a100` is two and two in-flight items carry it
- **THEN** a third item carrying it waits, while an item carrying no such
  tag may still be admitted

#### Scenario: Finishing frees a slot

- **WHEN** an in-flight item finishes while others wait
- **THEN** the next eligible item is admitted on the next pass, within the
  limits

#### Scenario: No limits, no bounding

- **WHEN** the fleet declares no concurrency limits
- **THEN** items are admitted as nodes will take them

### Requirement: Running an item

An admitted item SHALL run as a one-shot agent: the active harness, in its
non-interactive single-task form, given the item's instructions, working in
the item's directory, with its inference pointed at the gateway and the model
of the node the item was matched to. The agent's output SHALL be kept, per
item, beside the items file, so a finished or failed item can be read after
the fact. The orchestrator SHALL verify the item's directory exists before it
launches the agent; an item whose directory is missing SHALL be failed,
naming the item, and the rest of the backlog SHALL go on. Where the command
was given `--create-item-dirs`, the orchestrator SHALL create the missing
directory instead, and the item SHALL go on to launch; a directory it cannot
create SHALL fail the item, naming the item and the cause.

#### Scenario: An admitted item runs against the gateway

- **WHEN** the orchestrator admits an item matched to a node
- **THEN** its agent runs in the item's directory, one-shot, with its
  inference pointed at the gateway and the node's model

#### Scenario: A missing directory fails the item alone

- **WHEN** an admitted item's directory does not exist
- **THEN** the item is failed, naming it, and other items still run

#### Scenario: A missing directory is created where the command says to

- **WHEN** the command is given `--create-item-dirs` and an admitted item's
  directory does not exist
- **THEN** the orchestrator creates the directory and the item runs in it

#### Scenario: An item's output is kept

- **WHEN** an item's agent finishes, whatever its outcome
- **THEN** what the agent said is kept, per item, beside the items file

### Requirement: Item lifecycle and state

Each item SHALL move from the backlog to running to a finished end: done,
where its agent ended successfully, or failed, where it did not, or where it
could not be launched. The orchestrator SHALL keep this state beside the
items file, so that on starting it sees which items are done and which are
failed, and treats an item it last left running as failed, naming the
interruption, rather than re-running it. An item that fails SHALL NOT be
retried on its own: it stays failed until the operator acts. On a clean
interrupt, the orchestrator SHALL stop the agents it launched and put their
items back in the backlog, so no work is lost and none runs twice.

#### Scenario: An item's ends are recorded

- **WHEN** an item's agent ends successfully, and another's ends in error
- **THEN** the first is recorded done and the second failed, and both stay
  recorded across an orchestrator restart

#### Scenario: A restart does not re-run an item

- **WHEN** the orchestrator starts again and its state says an item was
  running when it last stopped
- **THEN** that item is recorded failed, naming the interruption, and is not
  re-run

#### Scenario: A clean interrupt loses no work

- **WHEN** the operator interrupts a running orchestrator
- **THEN** its agents are stopped, their items return to the backlog, and the
  next start picks them up again

### Requirement: What the orchestrator does not do

The orchestrator SHALL NOT start an engine on a node: the only thing that
ever does that is the gateway's behaviour for a request it holds. It SHALL
NOT publish an item's result anywhere — no pull request, no comment on the
work the item came from — beyond keeping the agent's output beside the items
file. It SHALL NOT hold a fleet file or any credential but the gateway's
token. Its limits SHALL bound how many items it admits, not which node the
fleet's gateway serves each request to.

#### Scenario: The orchestrator never starts a node

- **WHEN** an item is matched to a stopped node and admitted
- **THEN** the orchestrator launches the agent and nothing else, and it is
  the gateway, for the agent's first request, that starts the node

#### Scenario: A finished item is not published

- **WHEN** an item's agent ends successfully
- **THEN** the record of it is the item's state and kept output, and nothing
  is posted, opened, or committed on the item's behalf
