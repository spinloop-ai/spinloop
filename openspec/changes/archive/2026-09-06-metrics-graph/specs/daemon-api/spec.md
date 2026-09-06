## ADDED Requirements

### Requirement: Metrics endpoint reports the system-reading history

The metrics endpoint SHALL include, alongside the current readings, the
history of system readings the daemon retains: one entry per sampler tick
while an engine was running, each carrying its time and that tick's CPU,
memory, and per-GPU utilisation and memory readings. The history SHALL cover
at most the last 10 minutes at the sampler's cadence. It SHALL persist when
the engine stops — the readings taken before the stop remain, so a caller can
see what the engine was doing until it stopped — and SHALL be cleared when
the next engine starts, so one engine's readings are never reported against
another. Where no engine has run in this daemon's life, or no reading has been
taken, the field SHALL be omitted rather than empty.

#### Scenario: History grows while the engine runs

- **WHEN** an engine has been running for several sampler ticks and a metrics
  request is made
- **THEN** the response includes one history entry per tick, each stamped with
  its time, covering up to the last 10 minutes

#### Scenario: A stopped engine still reports its history

- **WHEN** a metrics request is made after the engine has been stopped
- **THEN** the response still includes the readings taken before the stop,
  though the current running-engine figures are omitted

#### Scenario: A new engine clears the previous history

- **WHEN** an engine is stopped and a later engine is started, and a metrics
  request is made
- **THEN** the history holds only the later engine's readings

#### Scenario: No history yet is absent, not empty

- **WHEN** a metrics request is made on a daemon that has never run an engine
- **THEN** the response carries no history field
