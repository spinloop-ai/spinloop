# Spec Delta

## MODIFIED Requirements

### Requirement: The detail view shows the node's metrics and log

The detail view SHALL be laid out as three stacked sections: the node's
metrics, the node's engine log, and a line naming the keys the view answers
to. The metrics section SHALL show the same facts, in the same wording, that
the node's tile and `fleet metrics` show for it, without the tile's clipping —
a fact that does not fit the tile SHALL be shown in full here. The metrics
section SHALL update on the node's normal refresh cadence, exactly as the
tile does.

The log section SHALL show the node's engine log, tailing and following it
the same way `spinloop fleet logs -f <node>` follows one node's log, so new
output appears while the view is open without the operator asking for it. A
node whose engine has not run yet, or that cannot supply its log, SHALL show
the same explanation the `fleet logs` command gives for it, rather than an
empty pane.

A log line longer than the view's width SHALL wrap onto continuation rows at
the left margin, so the whole line is readable within the view; wrapping
SHALL be on by default. The operator SHALL be able to turn wrapping off and
back on from the keyboard with the same key the serve view answers to; while
wrapping is off a longer line SHALL be cut at the view's width as before. The
view SHALL show whether wrapping is on or off. The pane's rows are then its
lines expanded to their wrapped rows, the newest rows at the pane's bottom.

The operator SHALL be able to pause and resume the log's follow from the
keyboard, independently of the rest of the view: while paused, the log
section SHALL stop picking up new output, and the view SHALL show whether the
log is following or paused. Nothing written while paused is lost: resuming
SHALL fetch and show whatever the engine wrote in the meantime, the same
backlog a poll would have picked up had it never paused. Pausing SHALL NOT
affect the metrics section's own refresh, or any node's start, stop or abort.

#### Scenario: Metrics match the tile

- **WHEN** the detail view is open on a node with a running engine
- **THEN** the metrics section shows the same state, serving facts, resource
  usage and counters the node's tile shows, in full rather than clipped

#### Scenario: The log pane follows new output

- **WHEN** the node's engine writes to its log while the detail view is open
- **THEN** the new lines appear in the log section without the operator
  pressing any key

#### Scenario: A line longer than the view wraps

- **WHEN** the log shows a line wider than the view's width
- **THEN** the line is drawn across as many rows as it needs, every row
  starting at the left margin, with no part of the line lost

#### Scenario: Turning wrapping off and on

- **WHEN** the operator turns wrapping off
- **THEN** lines longer than the view are cut at its width again, and the
  view shows that wrapping is off
- **AND** turning it back on wraps those lines again

#### Scenario: A node with no log yet

- **WHEN** the detail view is open on a node whose engine has never run
- **THEN** the log section shows the same explanation `fleet logs` gives for
  that node, not an empty pane

#### Scenario: Pausing the log

- **WHEN** the operator pauses the log's follow
- **THEN** the log section stops picking up new output and the view shows
  that the log is paused
- **AND** the metrics section keeps refreshing on its own cadence

#### Scenario: Resuming the log

- **WHEN** the operator resumes a paused log
- **THEN** whatever the engine wrote while paused appears in the log section,
  and new output continues to appear as it is written
