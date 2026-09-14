## Purpose

Work the work items file and the state the orchestrator keeps beside it from
the shell — add an item, read the backlog, stop a running item, remove an
item — whether or not the orchestrator is running, the file beside it being
the whole hand-off.

## Requirements

### Requirement: The work command group

`spinloop work` SHALL be a top-level command group with the subcommands add,
list, abort and remove. Every subcommand SHALL take an `--items` flag,
defaulting to `./work.yaml` in the working directory, naming the work items
file it works — the same file the orchestrator reads. A subcommand that
needs the file's current items SHALL fail before it changes anything where
the file is not a list of items, naming the file and the fault. The commands
SHALL NOT take the lock beside the file: an orchestrator running the same
file keeps its work while a command works it.

#### Scenario: The default file is the working directory's

- **WHEN** the operator runs a work command with no `--items` flag
- **THEN** it works `./work.yaml` in the working directory

#### Scenario: A file the command cannot parse changes nothing

- **WHEN** the operator runs a work command whose `--items` names a file that
  is not a list of items
- **THEN** the command fails, naming the file and the fault, and the file,
  the state and the logs are untouched

### Requirement: Adding an item

`spinloop work add` SHALL take the item's fields from flags — its id, its
instructions, its working directory, its tags, repeatable, and its numeric
priority — and append the item to the items file, the file a valid items file
after the append, with the validation the orchestrator applies: an item with
no instructions or no working directory is refused, naming the item, and a
tag that is not a `key=value` pair, or a key the item names twice, is
refused, naming the fault. An id the file already carries SHALL be refused,
naming it, and an id the state beside the file has recorded done or failed
SHALL be refused too, naming the record — an ended item is not worked again
under its own id. Where the items file does not exist, the command SHALL
create it, carrying the one item. A running orchestrator SHALL pick the new
item up on its next pass: the command writes the file and nothing more.

#### Scenario: An item's fields come from flags

- **WHEN** the operator runs add naming an id, instructions, a working
  directory, two tags, and a priority
- **THEN** the file carries the item with those fields, and the file is a
  valid items file

#### Scenario: A field the validation refuses

- **WHEN** the operator runs add with no working directory, or a tag that is
  not a `key=value` pair
- **THEN** the command fails, naming the item and the fault, and the file is
  untouched

#### Scenario: An id the file already has

- **WHEN** the operator runs add with an id the items file carries
- **THEN** the command fails, naming the id, and the file is untouched

#### Scenario: An id the state has recorded ended

- **WHEN** the operator runs add with an id the state beside the file has
  recorded done or failed
- **THEN** the command fails, naming the id and the record, and the file is
  untouched

#### Scenario: A missing file takes the first item

- **WHEN** the operator runs add where the items file does not exist
- **THEN** the file is created, carrying the one item

#### Scenario: A running orchestrator takes the item in

- **WHEN** the orchestrator is running the file and the operator adds an item
- **THEN** the orchestrator's next pass sees the item, and it is eligible for
  admission

### Requirement: Listing the work

`spinloop work list` SHALL show every item in the items file with its record
from the state beside it, in the file's order: each item's id, its state —
backlog, running, done or failed — the node a running item runs on, and when
it started and ended. An item with no record in the state SHALL show as
backlog, and a state file that does not exist at all — the orchestrator has
never run the file — SHALL mean every item is backlog. The command SHALL
work whether or not the orchestrator is running: it reads the file and the
state, and takes no lock. Its output SHALL be plain lines a program can
consume, one per item, and decoration SHALL be drawn only where there is a
terminal to draw it on. `ls` SHALL be an alias of list.

#### Scenario: Every item shows its state

- **WHEN** the file carries items the state records running, done, failed and
  not at all
- **THEN** the list shows them, in the file's order, running with its node
  and start, done and failed with their ends, and the unrecorded one backlog

#### Scenario: No state file, everything is backlog

- **WHEN** the operator lists a file the orchestrator has never run
- **THEN** every item shows as backlog

#### Scenario: A piped run gets plain lines

- **WHEN** the operator pipes the list into another program
- **THEN** the pipe carries one plain line per item, in order, with no
  decoration

#### Scenario: The alias works

- **WHEN** the operator runs `spinloop work ls`
- **THEN** it lists the work, the way `spinloop work list` does

### Requirement: Aborting an item

`spinloop work abort` SHALL take an item's id and stop the item: its agent
stopped the way the run stops one on a clean interrupt, and its record
removed from the state, so the item is back in the backlog and the run's next
pass admits it again. Only a running item SHALL be abortable: an id the file
does not carry is refused, naming it, and an item the state records backlog,
done or failed is refused too, naming the item and its state. Where no
orchestrator is running the file there is nothing running: the command SHALL
say so, and a stale running record left by an unclean death SHALL be left for
the next start to record failed, not cleared by abort. Where the orchestrator
is running, the command SHALL report the outcome: the agent stopped and the
item back in the backlog, waited for with a bound, naming the item where the
bound runs out first.

#### Scenario: A running item is stopped

- **WHEN** the orchestrator is running an item, and the operator aborts it
- **THEN** the item's agent is stopped, its record is gone from the state,
  and the run's next pass admits it again

#### Scenario: An item that is not running is refused

- **WHEN** the operator aborts an item the state records backlog, done or
  failed
- **THEN** the command fails, naming the item and its state, and nothing is
  stopped

#### Scenario: An id the file does not carry

- **WHEN** the operator aborts an id the items file does not carry
- **THEN** the command fails, naming the id

#### Scenario: No orchestrator, nothing running

- **WHEN** the operator aborts an item whose state says running, and no
  orchestrator is running the file
- **THEN** the command says so, and the stale record is left for the next
  start

#### Scenario: The command reports the outcome

- **WHEN** the operator aborts a running item
- **THEN** the command waits for the item to be back in the backlog, bounded,
  and says the item is stopped — or, where the bound runs out first, says so,
  naming the item

### Requirement: Removing an item

`spinloop work remove` SHALL take an item's id and remove it: the item out of
the items file, its record out of the state beside the file, and its kept
output gone from the logs beside it. The file SHALL stay a valid items file
after the removal. A running item SHALL NOT be removable: the refusal names
the item and says the abort that goes first. An id the file does not carry is
refused, naming it. Backlog, done and failed items are removed alike. A
running orchestrator SHALL pick the change up on its next pass: the item is
out of its backlog, and its record is dropped with it.

#### Scenario: An item is removed whole

- **WHEN** the operator removes an item that is backlog, done or failed
- **THEN** the file no longer carries it, the state no longer records it, and
  its kept output is gone

#### Scenario: A running item is refused

- **WHEN** the operator removes an item the state records running
- **THEN** the command fails, naming the item and that it aborts first, and
  nothing is removed

#### Scenario: An id the file does not carry

- **WHEN** the operator removes an id the items file does not carry
- **THEN** the command fails, naming the id, and nothing is removed

#### Scenario: A running orchestrator takes the change in

- **WHEN** the orchestrator is running the file and the operator removes an
  item
- **THEN** the orchestrator's next pass sees the item gone from the backlog,
  and the state no longer records it
