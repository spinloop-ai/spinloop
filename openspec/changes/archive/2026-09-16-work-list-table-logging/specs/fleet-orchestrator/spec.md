## MODIFIED Requirements

### Requirement: Serving the work list API

The orchestrator SHALL serve a work list API for the life of the run, on
the address its `--listen` flag names. The API's paths SHALL be:

- `GET /` — the list of the items file's items as the run holds them: each
  item's id, state, and the node it runs on, where it is running.
- `GET /log/<id>` — the output of the item named, where the run keeps it.
- `POST /` — a new item: the run admits it to the backlog, writes it to the
  items file, and the items file stays the source of truth.
- `DELETE /<id>` — an item out of the backlog: the run removes it from the
  items file and its state.
- `POST /abort/<id>` — an item out of the running: the run stops its agent,
  puts the item back in the backlog, and writes that to the items file and
  its state.

The API SHALL answer from the run's in-memory view of the items — the same
view the run's own loop reads and writes — and SHALL NOT re-read the items
file to answer a request. Where the run writes the items file, the write
SHALL carry the run's view, and SHALL NOT drop an item the API added or
remove one it removed. The run's loop and the API SHALL share one view of
the items: an item the API adds SHALL be eligible for admission on the
loop's next pass, and an item the loop finishes SHALL show finished to the
API.

The API SHALL authenticate its requests with a bearer token: where the run
listens on an address that is not loopback, every request SHALL present the
run's API token or be refused; where it listens on loopback, a request MAY
present no token and be served.

The orchestrator SHALL log every call the API answers, once the answer is
complete: the method, the path, the status the caller was given, and how
long the call took. A refusal or a fault SHALL be visible in the run's own
log at its default level, not only in the caller's reply.

#### Scenario: The run's view answers the list

- **WHEN** the run's loop admits an item and a request reads the list
- **THEN** the list shows the item running, on the node it was matched to

#### Scenario: A new item enters through the API

- **WHEN** a request posts an item to the API
- **THEN** the run's view holds it, the items file holds it, and the loop's
  next pass may admit it

#### Scenario: An item comes out through the API

- **WHEN** a request removes an item from the backlog, or aborts one that is
  running
- **THEN** the run's view no longer holds it in that state, the items file
  no longer holds it where it is removed, and an aborted item's agent is
  stopped and its item back in the backlog

#### Scenario: A stopped run's API is gone

- **WHEN** the run stops, whatever its cause
- **THEN** the API no longer answers

#### Scenario: A request without a token is refused off loopback

- **WHEN** the run listens on an address that is not loopback, and a request
  presents no token, or the wrong one
- **THEN** the request is refused, and no item is read or changed

#### Scenario: The loop and the API see one view

- **WHEN** the API adds an item while the loop's next pass runs
- **THEN** the pass sees the item, and an item the loop finishes shows
  finished to a request that reads it

#### Scenario: A refused call is logged

- **WHEN** a request is refused — a bad token, an id the file does not carry,
  a running item that cannot be added again
- **THEN** the run's log carries a record of the call naming its outcome,
  whether or not anyone is reading the caller's reply
