## MODIFIED Requirements

### Requirement: Repeated requests converge on one job

Requesting a seed that is already in flight SHALL join the existing job
rather than start a second one, and SHALL report that job's identity.
Convergence SHALL NOT depend on the caller having a lock or on the
requests being spaced apart: two requests arriving at the same moment
SHALL still produce one job. A seed whose compute has ceased — failed,
stopped or reaped — is not in flight: a request for its weights SHALL
start a fresh job for them rather than report the ceased seed as
running.

Requesting a seed for different weights SHALL start its own job, so
unrelated seeds proceed in parallel.

The number of jobs in flight at once SHALL be capped, counted over
seeds whose compute is alive, and a request that would exceed the cap
SHALL be refused with a reply that says so, so that a caller in a loop
cannot launch unbounded compute.

Once weights are present, a repeat request SHALL do nothing unless it
explicitly asks to seed them again, so that re-seeding is deliberate
rather than accidental.

#### Scenario: A second request joins the first

- **WHEN** a seed is requested for weights whose seed is already running
- **THEN** no second job is started, and the reply names the running job

#### Scenario: A stopped seed is not in flight

- **WHEN** a seed is requested for weights whose only seed instance is
  stopped
- **THEN** the stopped instance is not joined, and a new job is started
  for the weights

#### Scenario: Simultaneous requests produce one job

- **WHEN** two requests for the same weights arrive concurrently
- **THEN** exactly one job exists afterwards, and both replies name it

#### Scenario: Unrelated seeds run in parallel

- **WHEN** seeds are requested for two different models
- **THEN** each gets its own job

#### Scenario: The cap is enforced

- **WHEN** a request would exceed the cap on jobs in flight
- **THEN** it is refused, and the reply says the cap was reached

#### Scenario: A stopped seed holds no slot against the cap

- **WHEN** a stopped seed instance would otherwise occupy a slot of the
  cap on jobs in flight
- **THEN** the slot is not counted, and a request for new weights is not
  refused by it

#### Scenario: Re-seeding must be asked for

- **WHEN** a seed is requested for weights that are already present
- **THEN** nothing is started unless the request asks to seed them again
