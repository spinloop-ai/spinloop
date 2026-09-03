## ADDED Requirements

### Requirement: Reading a remote environment's logs resumes without duplicating events

A remote environment's log read SHALL resume from the position it last
returned rather than re-reading its whole tail on every call, so a fleet
node backed by a remote environment meets the same no-duplicate follow
guarantee a local node meets. It SHALL do so through the same follow cursor
`spinloop remote logs -f` uses — deduplicating by event id over a shared
overlap window — so the two follows cannot drift into different behavior.

A read that finds nothing SHALL distinguish two states: a from-the-beginning
read that finds nothing, meaning the engine has never logged here, and a
later read that finds nothing new, meaning the log is quiet rather than
missing. Only the former SHALL be reported as a missing log.

Opening a fresh follow of a node — for example, opening the fleet dashboard's
detail view on it — SHALL start the cursor over, so the fresh follow shows
its own tail rather than having it suppressed as already seen by a previous
follow of the same node.

#### Scenario: A follow does not repeat events already shown

- **WHEN** a remote node's log is polled repeatedly and the engine has
  written no new output between two polls
- **THEN** the second poll returns no content, not the same events again

#### Scenario: New output is shown once

- **WHEN** the engine writes new output between two polls
- **THEN** only the new output is returned by the next poll, not the output
  already returned by a previous one

#### Scenario: A quiet poll is not reported as missing

- **WHEN** a remote node's log has already shown output, and a later poll
  finds nothing new
- **THEN** the log is not reported as missing

#### Scenario: A log that has never shown output is reported as missing

- **WHEN** a remote node's log is polled for the first time and the engine
  has never logged anything
- **THEN** the log is reported as missing

#### Scenario: Reopening a follow shows the tail again

- **WHEN** a follow of a remote node's log is closed and reopened
- **THEN** the reopened follow shows the node's current tail, not an empty
  result because those events were already shown by the previous follow
