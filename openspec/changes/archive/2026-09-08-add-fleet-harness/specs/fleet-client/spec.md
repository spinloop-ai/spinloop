## ADDED Requirements

### Requirement: A fleet harness command

`spinloop fleet harness` SHALL configure the active harness for a fleet and
launch it: the fleet-level form of a fleet-routed launch, in which the fleet
file comes from the command rather than from a Spinloop's `FLEET`. The command
SHALL take a Spinloop the way `spinloop harness` does — an
`-O`/`--spinloop` argument, a leading alias or path, or the `Spinloop` beside
it — and a fleet file from `--fleet`/`-f`, defaulting to the `fleet.yaml`
beside it.

The command SHALL route the launch the way a fleet-routed launch routes: at
the gateway where the effective fleet file names one, otherwise by choosing a
node and, where the fleet's wake policy allows it, waking one — honouring
`--node`, `--prefer`, `--no-wake` and `--wake-timeout` as the launch does. The
choice SHALL be reported on stderr before the harness launches, as a
fleet-routed launch reports its choice.

Where the command is given no `-f`, a Spinloop that names a `FLEET` — a file
or an endpoint — SHALL be used, exactly as `--fleet` overrides an instruction
on `spinloop harness`; an explicit `-f` SHALL win over the instruction. A
Spinloop that pins a `BASEURL` SHALL NOT be routed, and a variable already set
in spinloop's environment SHALL win, in each case as on the launch path. A
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
  endpoint, as a fleet-routed launch would apply it

#### Scenario: The command's file wins over the Spinloop's FLEET

- **WHEN** the user runs `spinloop fleet harness -f ./a.yaml` with a Spinloop
  whose `FLEET` names a different file
- **THEN** the fleet in `./a.yaml` is the one the launch routes through

#### Scenario: No Spinloop, no route

- **WHEN** the user runs `spinloop fleet harness` in a directory holding no
  Spinloop and gives none
- **THEN** the command fails saying a launch needs a Spinloop to know which
  model to route, and no harness is launched
