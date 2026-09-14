# fleet-orchestrator Specification

## MODIFIED Requirements

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

The command SHALL serve the work list API the way `spinloop gateway` serves
its endpoint: on a `--listen` address whose default is a fixed port on every
interface, where a `--loopback` flag instead binds loopback on the default
port, and where an explicit address and `--loopback` given together fail the
command before it serves, naming both. The API's bearer token SHALL be
resolved the way the daemon's control API token is: a flag naming the value,
a flag naming a file to read it from, or the environment variable the daemon
uses — and a bind other than loopback with no token resolvable SHALL be
refused before the command serves, naming the address and the ways a token
may be supplied, while a loopback bind needs no token.

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

#### Scenario: Loopback needs no token

- **WHEN** the operator runs the command with `--loopback` and no token
  supplied
- **THEN** the work list API answers on loopback on the default port, with
  no token asked of callers

#### Scenario: A non-loopback bind without a token is refused

- **WHEN** the operator gives a `--listen` address other than loopback and
  no token is resolvable
- **THEN** the command fails before it serves, naming the address and the
  ways a token may be supplied

#### Scenario: An explicit address and loopback are a conflict

- **WHEN** the operator gives both an explicit `--listen` address and
  `--loopback`
- **THEN** the command fails before it serves, naming both

## ADDED Requirements

### Requirement: Serving the work list API

While it runs, the orchestrator SHALL serve the work list over HTTP: the
items file's items joined with the record the run keeps for each, and the
output each item's agent kept. Every path it serves SHALL sit behind the
bearer token the same way the gateway's paths do: a caller with the token
gets an answer, a caller without one is refused, and on a loopback bind with
no token a caller needs nothing. A method or path the API does not serve
SHALL be answered `404` naming the paths it does serve.

The work list SHALL show every item in the items file the run reads, in the
file's order, with its record: backlog where there is no record, running
with the node it runs on and when it started, done or failed with when it
ended and, for a failed one, why it failed. One item's kept agent output
SHALL be readable; an item with none is answered as having none, not as a
fault.

A caller SHALL be able to add an item — the fields the items file takes —
and the items file SHALL stay a valid items file after the add, with the
validation the orchestrator applies to the file: an id the file already
carries is refused, naming it, and an id the state records as done or failed
is refused too, naming the record. A caller SHALL be able to remove an item
— from the items file, its record, and its kept output — where it is not
running: removing a running item is refused, naming the item and saying to
abort it first, and removing an id the file does not carry is refused,
naming it. A caller SHALL be able to abort a running item: its agent is
stopped the way a clean interrupt stops it, and the item returns to the
backlog so the run may admit it again; aborting an item that is not running
is refused, naming the item and its state.

A change the API accepts SHALL reach the run on its next pass, the way a
change to the items file does, and nothing the API does SHALL leave the work
list in a state the run itself would not record.

#### Scenario: A caller with the token reads the work list

- **WHEN** a caller sends the API's token and asks for the work list
- **THEN** it gets every item in the items file, in the file's order, each
  with its record — or backlog where there is none

#### Scenario: A caller without the token is refused

- **WHEN** a caller sends no token, or the wrong one, to a path the API
  serves
- **THEN** it is refused the way the gateway's paths refuse it

#### Scenario: A running item shows its node and start

- **WHEN** an item is running and a caller reads the work list
- **THEN** that item shows running, the node it runs on, and when it
  started

#### Scenario: A caller reads an item's output

- **WHEN** a caller asks for an item's kept agent output
- **THEN** it gets what the agent said; an item with no kept output is
  answered as having none

#### Scenario: An added item enters the backlog

- **WHEN** a caller adds an item the file does not carry
- **THEN** the items file carries it, still a valid items file, and the run
  admits it on a later pass like any item it reads

#### Scenario: A duplicate id is refused

- **WHEN** a caller adds an item whose id the items file already carries
- **THEN** the add is refused, naming the id, and the file is unchanged

#### Scenario: A finished id is refused

- **WHEN** a caller adds an item whose id the state records as done or
  failed
- **THEN** the add is refused, naming the record, and the file is unchanged

#### Scenario: A removal takes the item, its record, and its output

- **WHEN** a caller removes an item that is not running
- **THEN** the items file no longer carries it, its record is gone from the
  state, and its kept output is gone with it

#### Scenario: A running item's removal is refused

- **WHEN** a caller removes an item that is running
- **THEN** the removal is refused, naming the item and saying to abort it
  first, and nothing is removed

#### Scenario: An abort returns the item to the backlog

- **WHEN** a caller aborts a running item
- **THEN** the item's agent is stopped, its record is gone from the state,
  and the run admits the item again on a later pass

#### Scenario: An abort of a non-running item is refused

- **WHEN** a caller aborts an item that is in the backlog, done, or failed
- **THEN** the abort is refused, naming the item and its state

#### Scenario: An unknown path is named as such

- **WHEN** a request is made to a method or path the API does not serve
- **THEN** the response is `404` and names the paths it does serve
