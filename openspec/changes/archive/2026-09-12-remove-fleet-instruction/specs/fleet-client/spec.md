## ADDED Requirements

### Requirement: The fleet harness command

`spinloop fleet harness` SHALL configure the active harness for a fleet and
launch it: the fleet-level form of a launch routed through a fleet, in which
the fleet file comes from the command rather than from the Spinloop. The
command SHALL take a Spinloop the way `spinloop harness` does — an
`-O`/`--spinloop` argument, a leading alias or path, or the `Spinloop` beside
it — and a fleet file from `--fleet`/`-f`, defaulting to the `fleet.yaml`
beside it.

The command SHALL route the launch the way a launch routed through a fleet
routes: at the gateway where the effective fleet file names one, otherwise by
choosing a node and, where the fleet's wake policy allows it, waking one —
honouring `--node`, `--prefer`, `--no-wake` and `--wake-timeout` as the launch
does. The choice SHALL be reported on stderr before the harness launches, as a
launch routed through a fleet reports its choice.

A Spinloop that pins a `BASEURL` SHALL NOT be routed, and a variable already
set in spinloop's environment SHALL win, in each case as on the launch path. A
command with no Spinloop to route SHALL fail before launching, saying that a
launch needs a Spinloop to know which model to route.

#### Scenario: A fleet's gateway is used

- **WHEN** the user runs `spinloop fleet harness` in a directory holding a
  fleet file that names a gateway and a Spinloop, and the gateway's token is
  set
- **THEN** the harness is applied for the Spinloop's model, the launched
  agent's endpoint is the gateway's address, and no node is contacted

#### Scenario: A fleet with no gateway routes to a node

- **WHEN** the user runs `spinloop fleet harness` against a fleet file that
  names no gateway, and a node is running the Spinloop's model
- **THEN** the harness is applied with that node's engine as the agent's
  endpoint, as a launch routed through a fleet would apply it

#### Scenario: The command's file wins over the fleet file beside it

- **WHEN** the user runs `spinloop fleet harness -f ./a.yaml` in a directory
  holding a `./fleet.yaml`
- **THEN** the fleet in `./a.yaml` is the one the launch routes through

#### Scenario: No Spinloop, no route

- **WHEN** the user runs `spinloop fleet harness` in a directory holding no
  Spinloop and gives none
- **THEN** the command fails saying a launch needs a Spinloop to know which
  model to route, and no harness is launched

## MODIFIED Requirements

### Requirement: Explaining a route

`spinloop fleet route` SHALL report the node a harness launch would choose for a
given Spinloop, the endpoint that node resolves to, and why it was chosen — and
SHALL change nothing: it SHALL never push a config, start an engine, or write a
harness config. It is how a routing decision is checked before an agent depends
on it, and how an unexpected choice is diagnosed after one.

When no node would be chosen, it SHALL report each node's state and the reason
it was passed over, and SHALL name what would happen on a real launch: which
node would be woken, or that none could serve it.

The Spinloop and the fleet file SHALL resolve as they do for a launch: the
Spinloop path defaults to `./Spinloop`, the fleet file comes from
`--fleet`/`-f`, and a Spinloop the user did not name explicitly picks up a
`./fleet.yaml` in the working directory. It SHALL accept `--prefer` and
`--node` as a launch does, and SHALL name the activity preference in force —
comparing the two preferences on a live fleet is the cheapest way to decide
which one a fleet should be run with. With no fleet file in force, the command
SHALL fail naming `--fleet`.

#### Scenario: The chosen node is explained

- **WHEN** `spinloop fleet route` runs against a fleet with a node serving the
  Spinloop's model
- **THEN** it prints that node, its resolved engine endpoint, and why it was
  chosen

#### Scenario: Routing changes nothing

- **WHEN** `spinloop fleet route` runs against a fleet where no node is serving
  the Spinloop's model
- **THEN** no engine is started, no config is pushed, and no harness config is
  written

#### Scenario: The two preferences can be compared

- **WHEN** `spinloop fleet route --prefer active` runs against a fleet whose file
  declares `prefer: idle`
- **THEN** it reports the node `active` would choose and names that preference,
  without changing the fleet file

#### Scenario: A launch that would wake a node says so

- **WHEN** `spinloop fleet route` runs and no node is serving the model but one
  could
- **THEN** it names the node a launch would wake, and does not wake it

#### Scenario: No fleet file names the flag

- **WHEN** `spinloop fleet route` runs on an explicitly named Spinloop in a
  directory holding no `fleet.yaml`, passing no `--fleet`
- **THEN** it fails saying there is no fleet file to route through, and names
  `--fleet`

## REMOVED Requirements

### Requirement: A fleet harness command

**Reason**: It treated the Spinloop's `FLEET` as a fleet source, and one of its
scenarios names that instruction; both are removed. The command itself stays,
restated without the `FLEET` tier.

**Migration**: See `The fleet harness command` in this capability.
