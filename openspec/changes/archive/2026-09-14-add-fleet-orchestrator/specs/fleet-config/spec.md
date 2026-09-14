# fleet-config Specification

## ADDED Requirements

### Requirement: Node tags

A node entry MAY declare `tags`: a mapping of key to value, both non-empty
strings. Tags are the operator's description of what work a node can take on —
capability (the model it serves, its context size), hardware, or anything else
the operator wants a dispatcher to match on. A tag is a claim the file makes
about the node: no node reads it, and it never changes what the node's engine
runs.

Tag keys SHALL be unique within a node's mapping; a duplicate key SHALL be
rejected at parse time. A node declaring no tags SHALL behave exactly as it
does today in every command, and SHALL match only work that names no tags.

#### Scenario: A node carries tags

- **WHEN** a fleet file's node entry declares `tags` with two key/value pairs
  and the file is read
- **THEN** both pairs are available, keyed, to the commands that report the
  fleet's topology

#### Scenario: A duplicate tag key is rejected

- **WHEN** a node's `tags` names the same key twice
- **THEN** parsing fails, naming the node and the duplicated key

#### Scenario: No tags, no change

- **WHEN** a fleet file's nodes declare no `tags`
- **THEN** nothing about the file's behaviour changes

### Requirement: Fleet-wide concurrency limits

A fleet file MAY declare a top-level `concurrency` section: how much work the
fleet may take at once. The section MAY set a `total`, the most work items the
fleet may have in flight at once, and a `tags` mapping from a tag (a
`key=value` pair, the way tags are named elsewhere) to the most items carrying
that tag that may be in flight at once, across the fleet.

It belongs to the file for the reason `wake` and `prefer` do: it describes how
this cluster is to be used — how much work its machines will take — which is a
property of the fleet, not of any one machine in it. The limits are the
fleet's declared capacity: a ceiling the operator sets, not a measurement of
the engines' load.

`total`, where declared, SHALL be a positive integer, as SHALL each per-tag
limit; a non-positive or non-integer value SHALL fail to parse, naming the
offending entry. A limit on a tag no node carries is accepted: it simply
bounds items that can match nothing. A file declaring no `concurrency`
section SHALL behave exactly as it does today.

#### Scenario: A fleet declares its limits

- **WHEN** a fleet file declares `concurrency` with a `total` and one per-tag
  limit, and the file is read
- **THEN** both limits are available, with the per-tag limit keyed by the tag
  it bounds

#### Scenario: A non-positive limit is rejected

- **WHEN** a fleet file declares a `concurrency` limit of zero or a negative
  number
- **THEN** parsing fails, naming the entry and what a limit must be

#### Scenario: No section, no change

- **WHEN** a fleet file declares no `concurrency` section
- **THEN** nothing about the file's behaviour changes
