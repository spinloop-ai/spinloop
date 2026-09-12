## MODIFIED Requirements

### Requirement: System readings for the retained history

While an engine is running, the sampler SHALL take one system reading — the
host's CPU, memory, and GPU figures — on each tick at its own interval,
independently of any request to the control API and independently of whether a
scrape target for the engine's counters is known. Each reading SHALL be
retained in the history the metrics endpoint reports, for at most the last
10 minutes.

One reading SHALL serve both what is retained and what is reported: the same
collection feeds the retained history and the current reading the metrics
endpoint answers with, so a request never takes a second collection. A reading
that yields no figure at all SHALL contribute no sample to the retained
history — a failed sample is a non-observation there as in the activity
record — and SHALL still become the current reading, carrying its errors: with
no collection taken on the request, the reading is the only place a broken
source is reported from.

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
- **THEN** no sample is recorded for that tick in the retained history, and
  the reading the metrics endpoint reports is the failed one, its errors
  included

#### Scenario: Reading stops with the engine, retention does not end

- **WHEN** the engine is stopped
- **THEN** no further system readings are taken, and the readings taken before
  the stop remain retained until the next engine starts
