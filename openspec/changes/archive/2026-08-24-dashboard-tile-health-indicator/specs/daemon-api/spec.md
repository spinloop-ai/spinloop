## ADDED Requirements

### Requirement: Status and metrics report engine readiness

While an engine is running, and its runner has a known way to check, the
daemon SHALL background-check whether the engine can actually answer
inference requests, distinct from the supervisor's own `running` state,
which the process reaching a `running` supervisor state does not guarantee —
the process can be alive while still loading weights.

The check SHALL be a health request against the engine's own endpoint,
accepting an authenticated response the same as an unauthenticated healthy
one, so a key-gated engine is not reported unready merely for being gated.

The result SHALL be reported on both `/v1/status` and `/v1/metrics`, on the
same terms `lastActiveAt`/`idleSeconds` already are: drawn from one shared
record, so a caller cannot see one answer from status and a different one
from metrics at the same moment.

The readiness field SHALL be absent — not `false` — when it does not apply:
the engine is not running, the runner has no known health-check convention,
or the daemon predates this check. A caller SHALL treat an absent field as
unknown, not as unready, so an older daemon or an unsupported runner is not
mistaken for a stuck one.

#### Scenario: A freshly started engine is not yet ready

- **WHEN** a status or metrics request is made while an engine has just
  started and has not yet answered its own health check successfully
- **THEN** the response reports the engine not ready

#### Scenario: A warmed-up engine reports ready

- **WHEN** a status or metrics request is made while the engine has answered
  its own health check successfully
- **THEN** the response reports the engine ready

#### Scenario: Status and metrics agree

- **WHEN** a status request and a metrics request are made against the same
  daemon at the same moment
- **THEN** both report the same readiness

#### Scenario: A gated engine is not penalised for requiring a key

- **WHEN** the running engine was started with an API key and its health
  check answers unauthenticated
- **THEN** the daemon still reports it ready, not unready

#### Scenario: A runner with no known health check reports no readiness

- **WHEN** the running engine's runner has no established health-check
  convention
- **THEN** the response carries no readiness field, rather than a false
  "not ready"

#### Scenario: An idle daemon reports no readiness

- **WHEN** a status or metrics request is made while no engine is running
- **THEN** the response carries no readiness field
