## ADDED Requirements

### Requirement: Prewarming the model's page cache

The daemon command SHALL accept a pre-warm option, off by default. When the
daemon runs with the option, starting an engine SHALL first read the model's
files — the deploy config's model path, whether a file or a directory —
sequentially, in the background, ahead of the engine's own copy to the GPU.
The read is best-effort: it SHALL NOT block the start, a failure SHALL be
silent, and a missing path SHALL be a no-op.

The option is a ceiling, not a default a caller may raise: a start request
SHALL be able to disable the pre-warm for that start, and SHALL NOT be able
to enable it on a daemon that does not run with the option. A daemon that
does not run with the option SHALL never pre-warm, whatever its stored
deploy config or any start request says.

#### Scenario: An opted-in daemon pre-warms

- **WHEN** the daemon runs with the pre-warm option and a start names a model
- **THEN** the model's files are read sequentially in the background as the
  engine starts, and the engine's copy to the GPU is served mostly from the
  page cache

#### Scenario: A plain daemon never pre-warms

- **WHEN** the daemon runs without the pre-warm option and a start names a
  model
- **THEN** no pre-warm read happens, and the engine starts exactly as it did
  before the option existed

#### Scenario: A start may disable the pre-warm

- **WHEN** the daemon runs with the pre-warm option and a start request's
  deploy config sets the pre-warm to disabled
- **THEN** that start does not pre-warm, and the daemon's option is
  unchanged for later starts

#### Scenario: A start cannot enable what the daemon has not

- **WHEN** the daemon runs without the pre-warm option and a start request's
  deploy config sets the pre-warm to enabled
- **THEN** no pre-warm read happens
