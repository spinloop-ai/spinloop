## ADDED Requirements

### Requirement: The pre-warm choice on a start

`outfit remote start` SHALL accept an explicit pre-warm choice — enabled or
disabled — and SHALL default to sending none, in which case the cloud default
applies. A start that carries the choice SHALL send it to the control plane,
which SHALL pass it to the daemon's start request for that wake. A restart,
which composes a stop and a start, SHALL accept the same choice and carry it
the same way.

#### Scenario: The default start pre-warms

- **WHEN** the user runs `outfit remote start` without a pre-warm choice
- **THEN** the wake sends no choice, and the engine's start pre-warms the
  page cache as the cloud default says

#### Scenario: A start may skip the pre-warm

- **WHEN** the user runs `outfit remote start` with the pre-warm disabled
- **THEN** the engine's start for that wake carries the pre-warm choice
  disabled, and the model loads without the pre-warm read

#### Scenario: A restart carries the choice too

- **WHEN** the user runs a restart with the pre-warm disabled
- **THEN** the restart's start of the instance carries the pre-warm choice
  disabled, exactly as a start would
