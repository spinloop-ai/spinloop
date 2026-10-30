# Spec Delta

## ADDED Requirements

### Requirement: Resolution of fleet and node targets
Every command that acts on a fleet — the `spinloop fleet` group, and the
top-level verbs that read one — SHALL resolve the fleet the same way, from the
flags given: `--node <name>` names a single registered cloud environment,
`--fleet <path>` names a fleet file, and with neither the fleet file is
`./fleet.yaml` in the working directory. One rule serves every such command, so
a target means the same thing wherever it is given.

Where no target resolves — no `--node`, no `--fleet`, and no fleet file at the
expected path — the command SHALL fail naming every way to give one: the
expected path, `--fleet <path>`, and `--node <name>`. It SHALL NOT look for an
engine on the machine it is running on. A local engine is already shown by the
command that runs it, and a machine whose engines are to be read this way names
them in a fleet file like any other.

`--fleet` names the fleet and `--node` names a node within it: paired, the
command SHALL narrow the named fleet to that node. A name the *named* file
does not hold SHALL fail naming the file and its known node names — the file
is the fleet the command has stated it acts on, and no other fleet answers
for it. On the launch path the pair means the same: `--node` there pins the
selection to that fleet's node (see the `fleet-routing` specification).

Beside a `fleet.yaml` discovered in the working directory — not named by a
flag — a `--node` that is a node of that fleet SHALL narrow the command to
that node alone; a name the fleet does not hold SHALL resolve as a registered
environment when one answers it, and fail naming the fleet and its known nodes
when nothing does. On the drive commands (`fleet start`, `fleet stop`) that
narrowing alone SHALL select the node: `-n <name>` stands in for the positional
node name rather than the command demanding it again.

`--node` SHALL name a registered environment, which is a plain identifier and
never a path. A value that is not a plain identifier, and a name with no
registered configuration, SHALL each fail saying so and how to create the
environment, before any node is contacted.

`--fleet` SHALL carry a `-f` short form on every `spinloop fleet` subcommand
except `fleet logs`, where `-f` is the short form of `--follow`; on `fleet
logs` the fleet file SHALL be named by the long form `--fleet` only. `--node`
SHALL carry the `-n` short form everywhere it appears except `fleet deploy`
and `cloud deploy`, where `-n` is the short form of `--dry-run`; on those two
the node SHALL be named by the long form `--node` only.

#### Scenario: Default resolution

- **WHEN** a `spinloop fleet` command runs in a directory containing
  `fleet.yaml` with no `--fleet` or `--node` flag
- **THEN** that file is used

#### Scenario: --node narrows the discovered fleet

- **WHEN** `spinloop status -n gpu-box` runs in a directory whose
  `fleet.yaml` holds a node named `gpu-box`, passing no `--fleet`
- **THEN** the command reads only that node: its row alone, its log alone,
  its engine alone

#### Scenario: A discovered fleet without the node falls back to the registry

- **WHEN** `spinloop status --node prod` runs in a directory whose
  `fleet.yaml` holds no node named `prod` and a registered environment named
  `prod` exists
- **THEN** the command acts on that environment's fleet of one

#### Scenario: A name nothing answers names the fleet it was checked against

- **WHEN** `spinloop status --node ghost` runs in a directory whose
  `fleet.yaml` holds no node named `ghost` and no environment named `ghost`
  is registered
- **THEN** the failure names `ghost`, the `fleet.yaml` it was not found in,
  and the fleet's known node names

#### Scenario: A drive command takes -n as the node name

- **WHEN** `spinloop fleet start -n gpu-box` runs in a directory whose
  `fleet.yaml` holds a node named `gpu-box`, naming no node positionally
- **THEN** that node alone is started, and the command does not ask for the
  name to be repeated

#### Scenario: Explicit path

- **WHEN** `spinloop fleet status --fleet ./cluster.yaml` runs
- **THEN** that file is used

#### Scenario: Short form

- **WHEN** `spinloop fleet status -f ./cluster.yaml` runs
- **THEN** `./cluster.yaml` is used, exactly as with `--fleet`

#### Scenario: logs keeps -f for follow

- **WHEN** the operator runs `spinloop fleet logs -f`
- **THEN** that is the command's follow flag, not a fleet-file flag, and
  `logs` takes its fleet file only as `--fleet`

#### Scenario: Missing file

- **WHEN** a `spinloop fleet` command runs with no fleet file at the resolved
  path
- **THEN** it fails, naming the expected path

#### Scenario: A named environment is the target

- **WHEN** `spinloop fleet status --node prod` runs, passing no `--fleet`
- **THEN** the command acts on the registered environment `prod` alone —
  whether or not a `fleet.yaml` is present, so long as no discovered fleet
  holds a node of that name, which would answer first

#### Scenario: A named environment needs no fleet file

- **WHEN** `spinloop fleet dashboard --node prod` runs in a directory holding
  no `fleet.yaml`
- **THEN** the view opens on that one environment, and the absent file is not
  an error

#### Scenario: A node of a named fleet narrows it

- **WHEN** `spinloop fleet status --node gpu-box --fleet ./cluster.yaml`
  runs and `cluster.yaml` holds a node named `gpu-box`
- **THEN** the command reads only that node — the row `gpu-box` alone, the
  file named by the flag supplying the fleet

#### Scenario: A name a named fleet lacks is not resolved elsewhere

- **WHEN** `spinloop fleet status --node prod --fleet ./cluster.yaml` runs,
  `cluster.yaml` holds no node named `prod`, and a registered environment
  named `prod` exists
- **THEN** it fails naming `prod`, `cluster.yaml` and its known node names,
  contacts nothing, and does not fall back to the registered environment —
  only a discovered `fleet.yaml` answers to the registry

#### Scenario: The directory's fleet file is read for a node, not ignored

- **WHEN** `spinloop fleet status --node prod` runs in a directory containing
  a `fleet.yaml` that holds a node named `prod`
- **THEN** the fleet is that file and the command narrows to the node
  `prod` alone, because beside a discovered fleet the one name answers one
  node of it, and only a flag states a conflict

#### Scenario: An unregistered environment is reported

- **WHEN** `spinloop fleet status --node nope` runs and no configuration is
  registered under that name
- **THEN** it fails saying the environment is not registered and how to
  create it, and contacts nothing

#### Scenario: A path is not an environment name

- **WHEN** `spinloop fleet status --node ./cloud.json` runs
- **THEN** it fails saying an environment name is a plain identifier with no
  path

#### Scenario: No target at all names every way to give one

- **WHEN** a command that acts on a fleet runs with no `--node`, no `--fleet`,
  and no fleet file at the expected path
- **THEN** it fails naming the expected path, `--fleet <path>` and
  `--node <name>`, and contacts nothing

#### Scenario: No engine on this machine is looked for

- **WHEN** a read verb runs with no target resolvable on a machine that is
  running an engine with its control API up
- **THEN** it fails as above rather than reporting that engine: a local engine
  is read through the command that runs it, or by naming the machine in a fleet
  file

## REMOVED Requirements

### Requirement: Fleet file resolution
**Reason**: The requirement's scenarios name the target flag in their titles; `--env`/`-e` is renamed `--node`/`-n`, and a MODIFIED cannot retitle scenarios.
**Migration**: Read the requirement "Resolution of fleet and node targets".


## MODIFIED Requirements

### Requirement: A registered environment is a fleet of one

A `--node <name>` target SHALL be a fleet holding exactly one node: a cloud
node named by the flag, whose registered configuration is the one
`clouds/<name>/cloud.json` holds. It SHALL be driven, observed and rendered
exactly as the same node listed in a fleet file is, so a command's output for
one environment does not depend on how that environment was named.

Such a fleet has no file on disk. It SHALL therefore carry none of the
settings a fleet file supplies — no activity preference, no wake policy, no
gateway, no concurrency limits, no fleet-wide key reference — and SHALL take
each of those as its default. Where a command reports which fleet it acted
on, it SHALL name the environment rather than a path.

A token reference SHALL NOT be resolved for such a fleet: a cloud node is
reached through its control plane with the operator's own credentials, and
there is no file for an adjacent `.env` to sit beside. An environment whose
credentials are missing or expired SHALL fail as it does on any other path.

#### Scenario: One environment renders as one node

- **WHEN** `spinloop fleet status --node prod` runs
- **THEN** the output is the one-row status table a single-node fleet file
  produces for the same environment

#### Scenario: An environment that cannot be reached is a row

- **WHEN** `spinloop fleet status --node prod` runs and the environment cannot
  be reached
- **THEN** the row reports why, and the command still succeeds, exactly as an
  unreachable node in a fleet file does

#### Scenario: Fleet-wide settings are absent, not inherited

- **WHEN** a command acts on a `--node` target
- **THEN** no activity preference, wake policy, gateway or concurrency limit
  is in force, and a `fleet.yaml` in the working directory supplies none of
  them

#### Scenario: No .env is consulted

- **WHEN** a command acts on a `--node` target
- **THEN** no adjacent `.env` is read, because the target names no file to sit
  beside
