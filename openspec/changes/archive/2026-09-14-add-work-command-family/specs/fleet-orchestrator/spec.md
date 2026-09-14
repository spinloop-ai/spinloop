## ADDED Requirements

### Requirement: The run drops a record the file no longer carries

The items file is the set of items the run works, so a record the file no
longer carries SHALL NOT stand: where an item has gone out of the file — by
an external edit, or by `spinloop work remove` — and is not in flight, the
run SHALL drop its record from the state on its pass, so the state holds no
record of an item the file has let go, and an id the file has let go can be
worked again where its record has stood ended. A record of an item still in
flight SHALL stand until the item ends, even where the file has let it go in
the meantime: the agent that is running runs to its end, and its record goes
with it out of the state once it is no longer in flight.

#### Scenario: A removed item's record goes

- **WHEN** an item that is backlog, done or failed goes out of the items file
  while the run works it
- **THEN** the run's pass drops its record from the state, and the state
  holds no record of it

#### Scenario: An in-flight item the file has let go

- **WHEN** an item that is running goes out of the items file while the run
  works it
- **THEN** the agent runs to its end, and the record is dropped from the
  state once the item is no longer in flight

#### Scenario: A let-go id can be worked again

- **WHEN** the state records an item failed, the item goes out of the file,
  and the run drops the record
- **THEN** the id can enter the file again and be worked

### Requirement: An abort marker stops a running item

Where a marker for an item stands beside the items file, asking the run to
abort it, the run SHALL take it up on its pass: the item's agent stopped the
way a clean interrupt stops it — the polite signal, the grace, then the hard
end — the item's record removed from the state, and the marker taken up, so
the item is back in the backlog. The run SHALL NOT admit the item again on
the pass that took the marker up: the item is eligible on the next pass. A
marker for an item that is not in flight SHALL be taken up without further
action — the marker goes, and nothing else changes.

#### Scenario: A marker stops the item

- **WHEN** a marker for a running item stands beside the file, and the run's
  pass comes
- **THEN** the item's agent is stopped, its record is gone from the state,
  and the marker is taken up

#### Scenario: The pass that aborts does not re-admit

- **WHEN** the run takes up a marker for an item on a pass
- **THEN** the item is not admitted on that pass, and is eligible on the next

#### Scenario: A marker for an item not in flight

- **WHEN** a marker stands beside the file for an item that is not running
- **THEN** the marker is taken up, and nothing else changes
