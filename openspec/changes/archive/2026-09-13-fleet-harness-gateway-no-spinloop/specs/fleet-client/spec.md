## MODIFIED Requirements

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
set in spinloop's environment SHALL win, in each case as on the launch path.

A command with no Spinloop to route, where the effective fleet file names no
gateway, SHALL fail before launching, saying that a launch needs a Spinloop to
know which model to route: a node can only be matched by the model a Spinloop
names.

A command with no Spinloop to route, where the effective fleet file names a
gateway, SHALL NOT fail. A gateway resolves the model per request rather than
by matching a node against one, so no Spinloop is needed to name one: the
command SHALL configure the active harness with a generic OpenAI-compatible
provider pointed at the gateway's base URL, its token resolved from the
environment the way a Spinloop routed at a gateway resolves one, and no
default model set.

The same applies to a Spinloop that is given but names neither a model nor an
alias, when routed at a gateway: the command SHALL NOT fail on that account
either, for the same reason.

Whenever the harness is configured this way — routed at a gateway, with no
model or alias to apply — the command SHALL query the gateway's `GET
/v1/models` and use the result as the harness's model list, so the harness has
something to choose from instead of an empty one. A failure to complete that
query (the gateway unreachable, timed out, or answering something unusable)
SHALL NOT fail the command: it SHALL warn and configure the harness with an
empty model list, on the same terms a launch already warns and carries on
when it cannot refresh a remote endpoint's key. This model-list population is
a capability of harnesses whose config format holds more than one model per
provider; a harness with no such concept is configured exactly as it is
today, with no model or alias.

The provider such a launch configures SHALL be named and keyed by the
gateway, not by the catalogue's shared generic id: its display name SHALL
lead with "Gateway" — not the catalogue engine's own generic label — followed
by the gateway's `name` where the fleet file's `gateway` section gives one,
or its address otherwise (e.g. "Gateway (dev-2)" or "Gateway
(localhost:4000)"), the same "<label> (<qualifier>)" shape a remote
environment already reads as (e.g. "llama.cpp (dev-2)"). This keeps a second
gateway from overwriting the first's configured block, and — since the word
a user actually searches a model picker for is "gateway" — lets them find it
at all.

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
  Spinloop and gives none, and the effective fleet file names no gateway
- **THEN** the command fails saying a launch needs a Spinloop to know which
  model to route, and no harness is launched

#### Scenario: No Spinloop, a gateway routes anyway

- **WHEN** the user runs `spinloop fleet harness` in a directory holding no
  Spinloop and gives none, and the effective fleet file names a gateway with
  its token set and models to list
- **THEN** the command does not fail: the active harness is configured with a
  generic OpenAI-compatible provider at the gateway's address, its model list
  populated from the gateway's `GET /v1/models`, no single default model set,
  and the harness launches against the gateway

#### Scenario: The gateway's model list cannot be fetched

- **WHEN** the same launch as above runs, but the gateway does not answer
  `GET /v1/models` before the command times out
- **THEN** the command does not fail: it warns on stderr and configures the
  harness with an empty model list, and the harness still launches against
  the gateway

#### Scenario: A harness with no model-list concept still launches

- **WHEN** the user runs `spinloop fleet harness` with no Spinloop given
  against a fleet file that names a gateway, and the active harness's config
  format holds at most one model per provider
- **THEN** the command does not fail: the harness is configured with the
  gateway's address and no model, whether or not the models query succeeds,
  since there is nowhere in that harness's config to put a list

#### Scenario: The gateway's own name labels the provider

- **WHEN** the user runs `spinloop fleet harness` with no Spinloop given
  against a fleet file whose `gateway` section names both a `url` and a
  `name`
- **THEN** the harness's provider is keyed and displayed using that name,
  reading the way a remote environment does (e.g. "Gateway (remote-llms)")

#### Scenario: An unnamed gateway is labelled by its address

- **WHEN** the same launch as above runs against a `gateway` section naming a
  `url` but no `name`
- **THEN** the harness's provider is keyed and displayed using the address's
  host instead
