# Spec Delta

## ADDED Requirements

### Requirement: Generation speed rides the sampling tick

While an engine is running, the sampler SHALL record a generation-speed point
in the retained history on each tick whose interval saw the engine emit a new
speed figure, carrying the most recent figure of that interval. A tick whose
interval saw no new figure SHALL carry no speed point: silence means the
engine generated nothing since the last tick, not that it generated at zero,
and a retained speed point is an observation, never a continuation.

A speed point SHALL make a tick's sample exist even where the system reading
that tick yielded no host figure at all — a generation rate is an observation
like any other. The speed points retained SHALL follow the history's existing
rules: they cover at most the last 10 minutes at the sampler's cadence, they
survive the engine's stop, and the next engine's start clears them with the
rest of the history.

#### Scenario: Steady generation puts a point on every tick

- **WHEN** an engine has been generating for several sampler ticks
- **THEN** the retained history carries a speed point for each of those ticks,
  each the latest figure its interval saw

#### Scenario: A quiet tick carries no speed point

- **WHEN** an engine is running but has emitted no timing line for the last
  tick's interval
- **THEN** that tick's history sample carries no speed point, while its host
  figures, if any, are still retained

#### Scenario: Speed alone still makes a sample

- **WHEN** a tick yields a new speed figure while the system reading yields no
  host figure at all
- **THEN** the tick's sample is retained, carrying the speed point

#### Scenario: Speed points survive a stop

- **WHEN** an engine that had been generating is stopped, and the history is
  read
- **THEN** the speed points taken before the stop are still retained, until a
  later engine starts and clears the history
