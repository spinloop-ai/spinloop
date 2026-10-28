# Spec Delta

## ADDED Requirements

### Requirement: Metrics reports generation speed

The metrics endpoint SHALL report the engine's last parsed generation speed, in tokens per second, alongside the token counters while the engine is running and its output has carried a timing figure since the last sampling tick. The reading SHALL be absent while the engine is stopped, and absent while a running engine has generated nothing since the last tick, rather than reported as zero or as its last known value: the figure answers "how fast is it generating right now", and a stopped or quiet engine has no answer.

The history entries SHALL additionally carry, per entry, an optional speed point for a tick whose interval saw a timing figure, on the same retention terms as the entry's other figures — surviving a stop and cleared by the next engine's start.

Both the current reading and the history points are optional on the wire: an engine that reports no speed, and a daemon that predates the figure, simply omit them, and a consumer treats their absence as "not reported", never as zero.

#### Scenario: A generating engine reports its speed

- **WHEN** a metrics request is made against a running engine that has emitted
  timing figures
- **THEN** the response carries the latest parsed tokens-per-second figure

#### Scenario: A stopped engine reports no current speed but keeps the history

- **WHEN** a metrics request is made after an engine that had been generating
  is stopped
- **THEN** the response carries no current speed figure, while the history
  entries taken during generation still carry their speed points

#### Scenario: A quiet engine lets the figure lapse

- **WHEN** a metrics request is made against a running engine whose last
  timing figure predates the last sampling tick
- **THEN** the response omits the current speed figure, the token counters
  remaining as they are

#### Scenario: An engine or daemon without the figure omits it

- **WHEN** a metrics request is made against an engine that emits no timing
  figure, or a daemon that predates the figure
- **THEN** the response carries no speed field and no history entry carries a
  speed point, and every other figure is reported as before
