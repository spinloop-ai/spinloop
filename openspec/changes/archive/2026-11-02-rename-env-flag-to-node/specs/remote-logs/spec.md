# Spec Delta

## MODIFIED Requirements

### Requirement: An environment's shipped logs are readable from the CLI

`spinloop logs --node <name>` SHALL print the logs an environment's instances have
shipped, without the operator needing to know the log group or stream naming,
open the AWS console, or connect to an instance. It SHALL select which
environment to read using the same `--node <name>` flag every other read verb
uses, so `spinloop logs --node <name>` and `spinloop status --node <name>` given
the same `--node` always speak about the same environment.

#### Scenario: Reading an environment's logs

- **WHEN** the operator runs `spinloop logs --node <name>` where `spinloop status --node <name>`
  would report on that environment
- **THEN** the log events that environment's instances shipped are printed
- **AND** the operator is not required to name a log group, stream, or instance

#### Scenario: Reading a named environment's logs

- **WHEN** the operator runs `spinloop logs --node dev-2`
- **THEN** `dev-2`'s logs are printed rather than any other environment's

#### Scenario: The cloud spelling names its replacement

- **WHEN** the operator runs `spinloop cloud logs`
- **THEN** it fails naming `spinloop logs --node <name>` as the command that
  replaced it

### Requirement: An environment's logs are readable after the instance is gone

Reading logs SHALL NOT depend on an instance being running, nor on the
environment's control endpoints answering. Logs SHALL be read from the durable
store the instances ship to, so the output of a boot that failed, or of an
instance that has since terminated, is still available.

#### Scenario: A terminated instance's logs are still readable

- **WHEN** an instance has produced logs and has since terminated
- **THEN** `spinloop logs --node <name>` still prints that instance's shipped events

#### Scenario: A stopped environment can be diagnosed

- **WHEN** an environment is stopped, so its status reports no running instance
- **THEN** `spinloop logs --node <name>` still prints the logs from its previous runs

### Requirement: An environment's both engine and boot logs are reachable

The command SHALL be able to read either log source an instance ships — the
inference engine's output and the boot (user-data) output — and both together.
The engine log SHALL be the default source, since it is what an operator wants
once the model is serving. Selecting the boot source SHALL be possible without
knowing which engine the environment runs, so a failure that happened before
the engine started is reachable even though the engine log is empty.

#### Scenario: Engine output by default

- **WHEN** the operator runs `spinloop logs --node <name>` with no source selected
- **THEN** the environment's engine log events are printed

#### Scenario: Boot output on request

- **WHEN** the operator asks for the boot source
- **THEN** the environment's start-up output is printed, including steps that
  run before the engine starts

#### Scenario: Both sources interleaved

- **WHEN** the operator asks for all sources
- **THEN** events from both the engine and boot logs are printed together in
  time order
- **AND** each line identifies which source it came from

#### Scenario: The engine need not be named

- **WHEN** an environment's logs are read and the operator has not stated which
  inference engine it runs
- **THEN** the engine's logs are found regardless of which supported engine
  produced them

### Requirement: An environment's the volume fetched is bounded and controllable

The command SHALL bound what it fetches by default rather than pulling an
environment's entire retained history, and SHALL let the operator widen or
narrow that: how far back to look, how many events at most to return, and
whether to restrict output to a single instance. When a bound causes older
events to be omitted, the command SHALL say so rather than presenting a
truncated view as complete.

#### Scenario: A default window applies

- **WHEN** the operator runs `spinloop logs --node <name>` with no window stated
- **THEN** only events from a bounded recent window are fetched

#### Scenario: The window is widened

- **WHEN** the operator states how far back to look
- **THEN** events from that whole period are fetched, subject to the retention
  of the durable store

#### Scenario: Output is capped

- **WHEN** more events match than the stated maximum
- **THEN** the most recent events up to that maximum are printed
- **AND** the operator is told that earlier matching events were omitted

#### Scenario: One instance is singled out

- **WHEN** the operator names an instance
- **THEN** only that instance's events are printed, and events from the
  environment's other instances are excluded
