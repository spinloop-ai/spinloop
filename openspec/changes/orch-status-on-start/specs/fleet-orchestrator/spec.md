## MODIFIED Requirements

### Requirement: The orchestrator command

`spinloop orchestrator` SHALL run as a long-running foreground process, the
way `spinloop gateway` and `spinloop serve` do: its lifecycle is the
process's, and it is a top-level command beside them.

It SHALL take the address of a fleet's gateway and the path of its work items
file. The gateway's address comes from an explicit flag, or — where no flag
is given — from the `gateway` section of a fleet file, the one an explicit
fleet flag names or the file in the working directory; the flag, where
given, wins. The items file is a flag with a default of `./work.yaml` in the
working directory. It SHALL authenticate to the gateway with a bearer token
resolved the way the fleet file's gateway section resolves one: a flag
naming the environment variable, defaulting to `OPENAI_API_KEY` — the
section's own variable where the gateway comes from the file and the flag
was not given — and, where the gateway comes from the file, the value read
from the process environment first, then the `.env` beside the fleet file.

The fleet file, where the command reads it, is read for the gateway's
address and the token's variable and nothing else: no node fact in it
reaches the run, and the gateway is the run's only view of the fleet. Where
neither a flag nor a readable fleet file's section names a gateway, the
command SHALL fail before it works an item, naming the flag and the file. A
gateway it cannot reach, or will not authenticate it to, SHALL stop the
command with a message naming the gateway.

The command SHALL take a `--create-item-dirs` flag, defaulting to false:
where it is set, the orchestrator creates an item's missing directory before
launching its agent; where it is not, a missing directory fails the item.

The command SHALL take a `--listen` flag, defaulting to loopback port 4010:
where it is set, the orchestrator serves the work list API on that address
for the life of the run. The command SHALL take a `--loopback` flag that
selects loopback port 4010: it SHALL refuse to run the API on an address
that is not loopback unless an API token is set, and the API token is a flag
or a flag naming the file that holds the token — it SHALL resolve the flag
first, then the file, and SHALL NOT write it anywhere. The API token is the
orchestrator's own: it is distinct from the gateway's token and is not
inherited from any environment variable the command itself uses.

Once the work list API is ready and before the run's first admission pass,
the command SHALL print the startup banner — the items file, the gateway,
the backlog count, and the work list API's address — followed by the work
list itself, in the form `spinloop work list` prints it: a table where
standard output is a terminal, plain tab-separated lines otherwise. The
list SHALL be the run's own view of the items, the one the work list API
would answer at that moment, so a restart's recovered state — an item
already recorded done, failed, or running — shows the way it would show to
a caller of the API.

#### Scenario: A gateway and an items file drive the command

- **WHEN** the operator runs the command naming a reachable gateway and an
  items file, with the gateway's token in the environment
- **THEN** it reads the fleet's topology from the gateway and works the
  items file's backlog

#### Scenario: A fleet file names the gateway

- **WHEN** the operator runs the command naming no gateway, and the fleet
  file — the one the fleet flag names, or the file in the working
  directory — names a gateway in its section
- **THEN** the command works against that gateway, authenticating under the
  section's token variable where the token flag was not given, the token's
  value read from the environment first, then the `.env` beside the file

#### Scenario: No gateway anywhere stops the command

- **WHEN** the operator names no gateway, and no readable fleet file — or
  none whose section names a gateway — is to hand
- **THEN** the command fails before it works an item, naming the flag and
  the file

#### Scenario: An unreachable gateway stops the command

- **WHEN** the operator runs the command naming a gateway that does not
  answer, or that refuses its token
- **THEN** the command fails, naming the gateway and what went wrong, and
  works no item

#### Scenario: A loopback API takes no token

- **WHEN** the operator runs the command with a loopback listen address, and
  no API token set
- **THEN** the work list API serves on that address, and its requests take
  no token

#### Scenario: A non-loopback API without a token refuses

- **WHEN** the operator runs the command with a non-loopback listen address,
  and no API token set
- **THEN** the command fails before it works an item, naming the token

#### Scenario: The API token is its own

- **WHEN** the operator runs the command against a gateway that has a
  token, with the API token set
- **THEN** the API authenticates on its own token, and the gateway's token
  is not what its requests present

#### Scenario: A listen address that conflicts is refused

- **WHEN** the operator gives both a loopback flag and a listen address, or
  an address that is not loopback with no token
- **THEN** the command fails before it works an item, naming the conflict

#### Scenario: Startup prints the work list on a terminal

- **WHEN** the command starts with standard output a terminal, and the
  items file carries items
- **THEN** after the startup banner, the command prints the work list as a
  table — a heading row and one aligned row per item, id, state, node,
  started and ended — the same table `spinloop work list` prints

#### Scenario: Startup prints the work list on a pipe

- **WHEN** the command's standard output is piped into another program
- **THEN** after the startup banner, the command prints one plain
  tab-separated line per item, in the items file's order, with no
  decoration

#### Scenario: A restart's recovered state shows at startup

- **WHEN** the command starts against an items file whose state beside it
  already records an item done, failed, or running from a prior run
- **THEN** the startup work list shows that item in its recorded state, not
  backlog
