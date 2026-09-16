## MODIFIED Requirements

### Requirement: Listing the work from the work list API

`spinloop work list` SHALL read the API's list and show every item the run holds
with its record, in the file's order: each item's id, its state — backlog,
running, done or failed — the node a running item runs on, and when it started
and ended. An item with no record SHALL show as backlog. The command's output
SHALL be plain lines a program can consume, one per item, and decoration SHALL
be drawn only where there is a terminal to draw it on: there, the columns
SHALL line up as a table, each column as wide as its widest value including
the heading, and the table SHALL open with a heading row naming the columns.
`ls` SHALL be an alias of list.

#### Scenario: Every item shows its state

- **WHEN** the API's list carries items the run records running, done, failed
  and not at all
- **THEN** the list shows them, in the file's order, running with its node and
  start, done and failed with their ends, and the unrecorded one backlog

#### Scenario: A piped run gets plain lines

- **WHEN** the operator pipes the list into another program
- **THEN** the pipe carries one plain line per item, in order, with no
  decoration

#### Scenario: A terminal gets a heading and aligned columns

- **WHEN** the operator runs list on a terminal, and an item's id is longer
  than a value already shown in that column
- **THEN** the table opens with a heading row naming the columns, and every
  row's columns still line up under it

#### Scenario: The alias works

- **WHEN** the operator runs `spinloop work ls`
- **THEN** it lists the work, the way `spinloop work list` does
