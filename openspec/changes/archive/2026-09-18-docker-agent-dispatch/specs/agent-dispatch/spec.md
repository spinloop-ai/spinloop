## Purpose

How the orchestrator runs an admitted item's harness process: the backend
it chooses, and what each backend guarantees the agent and its isolation
from the host. A bare process on the orchestrator's own host is one
backend; a docker container is another, with a stronger sandbox and a
remote host as later additions behind the same choice.

## ADDED Requirements

### Requirement: Choosing the dispatch backend

`spinloop orchestrator` SHALL take a `--dispatch` flag naming the backend
every launch the run makes uses, one of `bare` or `docker`. Where the flag
is not given, harness.yaml's own `dispatch` (see "Configuring the harness
with harness.yaml") SHALL be the run's choice where it names one; where
neither says anything, the run SHALL use `bare`. A name neither the flag
nor harness.yaml's field recognises SHALL fail the command before it works
an item, naming whichever of the two named it, the value, and the accepted
set. The choice is the run's: every item the run launches goes through the
same backend, not a choice per node or per item.

#### Scenario: The default is the bare backend

- **WHEN** the operator runs the command with no `--dispatch` flag and no
  harness.yaml naming a dispatch
- **THEN** every item it admits runs through the bare backend

#### Scenario: Docker is named

- **WHEN** the operator runs the command with `--dispatch docker`
- **THEN** every item it admits runs through the docker backend

#### Scenario: harness.yaml names the backend

- **WHEN** the operator runs the command with no `--dispatch` flag, and
  harness.yaml carries a `dispatch`
- **THEN** every item it admits runs through the backend harness.yaml
  names

#### Scenario: An explicit flag wins over harness.yaml's

- **WHEN** the operator runs the command with `--dispatch`, and
  harness.yaml also carries a `dispatch` naming a different backend
- **THEN** every item it admits runs through the flag's backend

#### Scenario: An unrecognised backend stops the command

- **WHEN** the operator runs the command with `--dispatch`, or harness.yaml
  with no `--dispatch` given, naming anything but `bare` or `docker`
- **THEN** the command fails before it works an item, naming whichever of
  the two named it, the value, and the accepted set

### Requirement: The bare backend

The bare backend SHALL run an admitted item's harness as a process on the
orchestrator's own host, in its own process group, working in a
`workspace` subdirectory of the item's own directory (created fresh on
every launch, if it is not already there), its config the harness's own
host config file, written with the node's provider before the launch the
way it always has been. Stopping the item SHALL stop the process group:
the polite signal first, then, where the grace runs out, the hard end.

#### Scenario: An item runs as a bare process

- **WHEN** the run admits an item under the bare backend
- **THEN** the harness runs as a process on the orchestrator's host,
  working in the item's own `workspace` subdirectory, its inference
  pointed at the gateway

### Requirement: The docker backend

The docker backend SHALL run an admitted item's harness inside a container
from the official spinloop agent image. A gateway address bound to the
orchestrator host's own loopback SHALL reach the container by
`host.docker.internal` instead — never a change to the fleet file or the
gateway flag — so it answers from inside the container the way it does a
bare process; a gateway already on a routable address is reachable from
the container's own network unchanged. The container SHALL carry two
mounts, both from subdirectories of the item's own directory (created
fresh where not already there): a `workspace` subdirectory as the
harness's working directory, and a `config` subdirectory holding a
config generated fresh for that one launch, carrying only that launch's
provider — never the orchestrator host's own harness configuration —
mounted where the harness resolves its own config to inside the
container. The token the provider needs SHALL reach the container through
its environment, the way the bare backend's does. Stopping the item SHALL
stop the container: the polite signal first, then, where the grace runs
out, the hard end. A docker failure — the daemon unreachable, the image
missing, the container refusing to start — SHALL fail the item alone,
naming the item and the cause, the rest of the backlog going on.

#### Scenario: An item runs inside a container

- **WHEN** the run admits an item under the docker backend
- **THEN** a container starts from the official agent image, the item's
  own `workspace` subdirectory mounted as the harness's working
  directory, and the harness's inference reaches the gateway

#### Scenario: A loopback gateway reaches the container

- **WHEN** the gateway is bound to the orchestrator host's own loopback,
  and the run admits an item under the docker backend
- **THEN** the container's harness reaches it by `host.docker.internal`,
  with nothing changed in the fleet file or the gateway flag

#### Scenario: The container's config carries only this launch

- **WHEN** the run admits an item under the docker backend
- **THEN** the container's mounted config carries this launch's provider
  alone, and no other configuration the orchestrator host's own harness
  config carries

#### Scenario: Stopping a containerized item

- **WHEN** a containerized item is aborted, or the run's own shutdown
  stops it
- **THEN** the container is stopped the polite way first, and killed where
  the grace runs out before it has

#### Scenario: A docker failure fails the item alone

- **WHEN** the docker daemon does not answer, or the agent image is not
  present, when the run tries to launch an item under the docker backend
- **THEN** the item is failed, naming the item and the cause, and the rest
  of the backlog goes on

### Requirement: The official agent image

Spinloop SHALL publish an image the docker backend's containers run from,
carrying opencode and Pi's one-shot forms and the runtimes they need, the
`gh` CLI, and none of any operator's own harness configuration: a
container from it, before the docker backend mounts a launch's config in,
has no provider configured.

#### Scenario: The image carries no baked-in configuration

- **WHEN** a container starts from the official agent image with nothing
  else mounted in
- **THEN** the harnesses it carries have no provider configured

#### Scenario: gh is on the image

- **WHEN** a container starts from the official agent image
- **THEN** `gh` runs in it, for an item's instructions that want it

### Requirement: Configuring the harness with harness.yaml

`spinloop orchestrator` SHALL read `harness.yaml` beside the items file by
default, or the file `--harness-config` names where the flag is given;
where neither the named file nor the default is present, the run SHALL
proceed exactly as it does without one. The file MAY carry a `dispatch`,
naming the backend the way `--dispatch` does — see "Choosing the dispatch
backend" for how the two are reconciled. The file MAY carry a `harness`,
naming the harness the way `--harness`/`-H` does; an explicit `--harness`
wins over it outright, and where neither says anything the harness is
resolved the way it always has been (the `HARNESS` environment variable,
then the stored preference, then the default). The file MAY carry a
`baseDir`: an item's own `dir`, where it is not already absolute, SHALL
resolve against it rather than the directory the orchestrator command
happens to be started from; a relative `baseDir` itself resolves against
`harness.yaml`'s own directory. The file MAY carry an `env` map, each
entry added to every launch's environment, under both backends alike. An
entry naming the variable the resolved token is presented under SHALL be
refused, naming it, before the command works an item. The file MAY also
carry `startup` and `shutdown`, each a multiline shell script — see "The
startup and shutdown scripts".

#### Scenario: No harness.yaml changes nothing

- **WHEN** the operator runs the command with no `--harness-config` flag
  and no `harness.yaml` beside the items file
- **THEN** every launch proceeds exactly as it would with no harness.yaml
  at all

#### Scenario: harness.yaml names the harness

- **WHEN** the operator runs the command with no `--harness` flag, and
  harness.yaml carries a `harness`
- **THEN** every item it admits runs against the harness harness.yaml
  names

#### Scenario: An explicit harness flag wins over harness.yaml's

- **WHEN** the operator runs the command with `--harness`, and
  harness.yaml also carries a `harness` naming a different one
- **THEN** every item it admits runs against the flag's harness

#### Scenario: harness.yaml's baseDir resolves a relative item dir

- **WHEN** harness.yaml carries a `baseDir`, and an admitted item's `dir`
  is relative
- **THEN** the item's directory is that `dir` resolved against `baseDir`,
  not the directory the orchestrator command was started from

#### Scenario: baseDir does not affect an absolute item dir

- **WHEN** harness.yaml carries a `baseDir`, and an admitted item's `dir`
  is already absolute
- **THEN** the item's directory is that `dir`, unchanged

#### Scenario: harness.yaml's env reaches the launch

- **WHEN** harness.yaml carries an `env` entry
- **THEN** every item's launch, under either backend, carries that
  variable in its environment

#### Scenario: An env entry colliding with the token is refused

- **WHEN** harness.yaml's `env` names the same variable the resolved
  token is presented under
- **THEN** the command fails before it works an item, naming the variable

#### Scenario: An explicit file is read instead of the default

- **WHEN** the operator runs the command with `--harness-config` naming a
  file
- **THEN** that file is read, whether or not `harness.yaml` exists beside
  the items file

### Requirement: The startup and shutdown scripts

A `startup` script SHALL run before the harness, under both backends
alike, in the item's own `workspace` subdirectory — the container's
`/item/workspace` for the docker backend — under the launch's full
environment — harness.yaml's `env` and everything the launch already
sets. A `startup` script that exits non-zero SHALL fail the item, naming
the script's failure, before the harness ever runs.

A `shutdown` script SHALL run once the harness has ended — a clean finish,
a failure, or an abort alike — provided a `startup` script ran at all,
whether or not that script itself succeeded: shutdown exists to tear down
whatever startup set up, which a failed startup may have partly done.
Where the item is aborted, the run's own answer to the abort SHALL wait
for the shutdown script the way it already waits for the harness itself,
bound by the same grace.

Both scripts' output SHALL join the harness's own in the item's kept log,
in the order they ran: startup's, then the harness's, then shutdown's.

#### Scenario: Startup runs before the harness

- **WHEN** harness.yaml names a startup script and the run admits an item
- **THEN** the script runs before the harness, in the launch's environment

#### Scenario: A failing startup script fails the item

- **WHEN** the startup script exits non-zero
- **THEN** the item is failed, naming the script's failure, and the
  harness never runs

#### Scenario: Shutdown runs after a clean finish

- **WHEN** harness.yaml names a shutdown script and the harness ends on
  its own
- **THEN** the shutdown script runs before the item is recorded done or
  failed

#### Scenario: Shutdown runs after an abort

- **WHEN** harness.yaml names a shutdown script and a running item is
  aborted
- **THEN** the harness is stopped, the shutdown script then runs, and the
  abort is answered once it has, bound by the same grace the harness's own
  stop already has

#### Scenario: Shutdown runs even where startup failed

- **WHEN** harness.yaml names both scripts and startup exits non-zero
- **THEN** the shutdown script still runs before the item is recorded
  failed

#### Scenario: Script output joins the kept log

- **WHEN** harness.yaml names a startup or shutdown script
- **THEN** the script's own output is in the item's kept log, in the order
  it ran relative to the harness's own output

### Requirement: An item's own copy of its log

Once an admitted item's agent has ended — whatever the outcome — its kept
log SHALL also be copied into the item's own directory, under both
backends alike. This copy is not live: while the agent runs, the item's
kept log beside the items file is the one that streams; the item
directory's own copy is written once, after the agent ends.

#### Scenario: The log is copied once the agent ends

- **WHEN** an admitted item's agent ends, under either backend
- **THEN** the item's own directory carries a copy of the item's kept
  log, matching it
