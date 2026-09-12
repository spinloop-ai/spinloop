## ADDED Requirements

### Requirement: Metrics reports the host's last sampled reading

The metrics endpoint's host figures — the GPU, CPU and memory readings — SHALL
be the background sampler's last reading, not a collection taken on the
request: one collection per tick serves the retained history and the reported
reading together, and reading the host's CPU costs a command that takes longer
the busier the host is, so a request that collected inline would block on
exactly the machine a caller is watching. A reported figure SHALL be at most
one sampling interval stale.

Before the first reading has landed, the host figures SHALL be absent rather
than zero or a fresh collection. A start after a stop SHALL drop the previous
engine's reading, so a new engine does not report the host's figures as they
stood for the last one. A reading's collection failures SHALL be reported among
the response's errors, naming the source: with no collection taken on the
request, the reading is the only place a broken source is reported from.

#### Scenario: A running engine's metrics report the sampled reading

- **WHEN** an engine has been running long enough for a sampling tick, and a
  metrics request is made
- **THEN** the response's host figures are the sampler's last reading, at most
  one sampling interval old

#### Scenario: No reading yet, no figures

- **WHEN** an engine has just started and no sampling tick has landed a
  reading, and a metrics request is made
- **THEN** the response carries no host figures

#### Scenario: A failed reading reports its errors

- **WHEN** the sampler's last system reading failed, and a metrics request is
  made
- **THEN** the response reports the failure among its errors, naming the
  source, and carries no figures the reading did not yield

#### Scenario: A start after a stop drops the previous reading

- **WHEN** an engine is stopped and a new engine is started, and a metrics
  request is made before the new engine's first tick
- **THEN** the response carries no host figures from the previous engine
