## MODIFIED Requirements

### Requirement: Running an item

An admitted item SHALL run as a one-shot agent: the active harness, in its
non-interactive single-task form, given the item's instructions, working in
a `workspace` subdirectory of the item's own directory, with its inference
pointed at the gateway and the model of the node the item was matched to.
The agent's output SHALL be kept, per item, beside the items file, so a
finished or failed item can be read after the fact. The orchestrator SHALL
verify the item's directory exists before it launches the agent; an item
whose directory is missing SHALL be failed, naming the item, and the rest
of the backlog SHALL go on. Where the command was given
`--create-item-dirs`, the orchestrator SHALL create the missing directory
instead, and the item SHALL go on to launch; a directory it cannot create
SHALL fail the item, naming the item and the cause.

#### Scenario: An admitted item runs against the gateway

- **WHEN** the orchestrator admits an item matched to a node
- **THEN** its agent runs in a `workspace` subdirectory of the item's
  directory, one-shot, with its inference pointed at the gateway and the
  node's model

#### Scenario: A missing directory fails the item alone

- **WHEN** an admitted item's directory does not exist
- **THEN** the item is failed, naming it, and other items still run

#### Scenario: A missing directory is created where the command says to

- **WHEN** the command is given `--create-item-dirs` and an admitted item's
  directory does not exist
- **THEN** the orchestrator creates the directory, and the item's agent
  runs in its workspace subdirectory

#### Scenario: An item's output is kept

- **WHEN** an item's agent finishes, whatever its outcome
- **THEN** what the agent said is kept, per item, beside the items file
