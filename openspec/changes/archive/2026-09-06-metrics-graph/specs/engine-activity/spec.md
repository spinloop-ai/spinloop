## ADDED Requirements

### Requirement: System readings for the retained history

While an engine is running, the sampler SHALL take one system reading — the
host's CPU, memory, and GPU figures — on each tick at its own interval,
independently of any request to the control API and independently of whether a
scrape target for the engine's counters is known. Each reading SHALL be
retained in the history the metrics endpoint reports, for at most the last
10 minutes. A failed system reading SHALL record no sample for its tick and
SHALL NOT be reported as an error: the on-request collection keeps its own
error reporting, and a transient sampling failure is neither data nor a
condition worth surfacing on every tick.

#### Scenario: System readings happen without being asked

- **WHEN** an engine is running and no client calls the control API
- **THEN** the daemon still takes a system reading on each sampler tick and
  retains it

#### Scenario: System readings do not depend on a scrape target

- **WHEN** the running engine exposes no metrics endpoint to scrape
- **THEN** the system readings are still taken and retained, since they come
  from the host, not from the engine

#### Scenario: A failed system reading records nothing

- **WHEN** a system reading fails on a tick because a host command is missing
  or fails
- **THEN** no sample is recorded for that tick and no error is reported for
  it

#### Scenario: Reading stops with the engine, retention does not end

- **WHEN** the engine is stopped
- **THEN** no further system readings are taken, and the readings taken before
  the stop remain retained until the next engine starts
