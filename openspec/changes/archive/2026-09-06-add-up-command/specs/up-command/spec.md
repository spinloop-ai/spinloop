## Purpose

`spinloop up`: the one-word command that starts the engine for what is in the
current directory — the fleet a `fleet.yaml` names, or the local engine a
Spinloop describes — dispatching to `spinloop fleet start` or `spinloop serve`
rather than reimplementing either.

## ADDED Requirements

### Requirement: Choosing the target from the directory

`spinloop up` SHALL start the fleet when the current directory holds a
`fleet.yaml`, and SHALL start the local engine a Spinloop describes otherwise.
A `fleet.yaml` SHALL win over a Spinloop in the same directory: when both are
present, `up` starts the fleet and the Spinloop is ignored. `up` SHALL take
positional arguments only, and a flag it does not know SHALL be refused as
unknown rather than forwarded or silently ignored.

#### Scenario: A fleet directory with both starts the fleet

- **WHEN** the user runs `spinloop up` in a directory holding a `fleet.yaml`
  and a `Spinloop`
- **THEN** the fleet's nodes are started and the local `Spinloop` is not read

#### Scenario: An unknown flag is refused

- **WHEN** the user runs `spinloop up --all` in a fleet directory
- **THEN** the command fails naming the unknown flag, and no node is started

### Requirement: Starting the fleet

In a directory holding a `fleet.yaml`, `spinloop up [node…]` SHALL start the
engines of the fleet's nodes as `spinloop fleet start` does: the named nodes
when any are given, and every node in the fleet when none are — a bare `up`
SHALL start the whole fleet, where a bare `fleet start` refuses to run without
a target. An unknown node name, an unreadable or node-less `fleet.yaml`, and
the per-node result lines SHALL be `fleet start`'s own.

#### Scenario: A bare up starts every node

- **WHEN** the user runs `spinloop up` in a fleet directory, with no node
  names
- **THEN** the same nodes are started, and reported the same way, as
  `spinloop fleet start --all` would start them

#### Scenario: Named nodes only

- **WHEN** the user runs `spinloop up gpu-box` in a fleet directory naming
  several nodes
- **THEN** only `gpu-box`'s engine is started, as `spinloop fleet start
  gpu-box` would do

#### Scenario: An unknown node name

- **WHEN** the user runs `spinloop up no-such-node` in a fleet directory
- **THEN** the command fails naming the known nodes, as `fleet start` does,
  and no node is started

#### Scenario: A broken fleet file

- **WHEN** the directory's `fleet.yaml` is unreadable or names no nodes
- **THEN** `up` fails with the fleet file's own error, as `fleet start` does

### Requirement: Starting the local engine

In a directory holding no `fleet.yaml`, `spinloop up [path]` SHALL behave
exactly as `spinloop serve [path]`: it SHALL resolve the Spinloop the same
way — an explicit path, a registered alias, the `SPINLOOP_ALIAS` variable,
then `./Spinloop` — print the resolved command, and run the engine with the
same output and the same errors. A directory where `serve` resolves no
Spinloop SHALL fail `up` with serve's own "no Spinloop found" error, naming
the same repairs.

#### Scenario: A bare up serves the directory's Spinloop

- **WHEN** the user runs `spinloop up` in a directory holding a `Spinloop` and
  no `fleet.yaml`
- **THEN** the engine is printed and started exactly as `spinloop serve`
  would do

#### Scenario: A path is served

- **WHEN** the user runs `spinloop up path/to/Spinloop` in a directory with no
  `fleet.yaml`
- **THEN** that Spinloop is served, as `spinloop serve path/to/Spinloop`
  would do

#### Scenario: The environment's alias resolves

- **WHEN** the directory holds no `./Spinloop` but `SPINLOOP_ALIAS` names a
  registered alias
- **THEN** that Spinloop is served: a directory where `serve` works is a
  directory where `up` works

#### Scenario: A registered alias resolves

- **WHEN** the user runs `spinloop up qwen` and `qwen` is a registered alias
- **THEN** the Spinloop it names is served, as `spinloop serve qwen` would do

#### Scenario: Nothing resolvable

- **WHEN** the user runs `spinloop up` in a directory with no `Spinloop`, no
  `SPINLOOP_ALIAS`, and no argument
- **THEN** the command fails with serve's "no Spinloop found" error, naming a
  path, an alias, and `SPINLOOP_ALIAS` as serve does

### Requirement: Completion offers what the branch accepts

`up`'s tab completion SHALL offer the fleet's node names when the current
directory holds a `fleet.yaml`, and otherwise the Spinloop slot — registered
alias names plus paths. When the fleet file cannot be read, completion SHALL
stay silent: no candidates, no stderr, no error.

#### Scenario: Node names in a fleet directory

- **WHEN** the user completes `spinloop up <TAB>` in a directory holding a
  `fleet.yaml`
- **THEN** the fleet's node names are offered

#### Scenario: The Spinloop slot elsewhere

- **WHEN** the user completes `spinloop up <TAB>` in a directory with no
  `fleet.yaml`
- **THEN** registered alias names and paths are offered, as on the other
  Spinloop commands

#### Scenario: An unreadable fleet file stays quiet

- **WHEN** completion is attempted in a directory whose `fleet.yaml` cannot be
  read
- **THEN** no candidates are offered and nothing is written to stderr
