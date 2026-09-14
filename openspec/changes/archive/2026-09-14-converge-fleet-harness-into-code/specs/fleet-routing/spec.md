## ADDED Requirements

### Requirement: A launch through a fleet's gateway needs no Spinloop

A launch routed through a fleet SHALL NOT require a Spinloop when the
effective fleet file declares a `gateway` section. A gateway resolves the model
per request rather than by matching a node against one, so there is no model
for a Spinloop to name; requiring one would ask for a file whose contents the
launch then ignores.

Where the fleet file names no gateway, a launch with no Spinloop SHALL fail
before launching, saying that `--fleet` needs a Spinloop because it is the
Spinloop's model that decides which node can serve you: a node can only be
matched by the model a Spinloop names. Where the named fleet file cannot be
read at all, the failure SHALL name that file rather than the missing
Spinloop — the file is the thing the launch was pointed at.

The same applies to a Spinloop that is given but names neither a model nor an
alias, when routed at a gateway: the launch SHALL NOT fail on that account
either, for the same reason.

Whenever the harness is configured this way — routed at a gateway, with no
model or alias to apply — the launch SHALL query the gateway's `GET /v1/models`
and use the result as the harness's model list, so the harness has something to
choose from instead of an empty one. A failure to complete that query (the
gateway unreachable, timed out, or answering something unusable) SHALL NOT fail
the launch: it SHALL warn and configure the harness with an empty model list,
on the same terms a launch already warns and carries on when it cannot refresh
a remote endpoint's key. This model-list population is a capability of
harnesses whose config format holds more than one model per provider; a harness
with no such concept is configured with no model or alias.

The provider such a launch configures SHALL be named and keyed by the gateway,
not by the catalogue's shared generic id: its display name SHALL lead with
"Gateway" — not the catalogue engine's own generic label — followed by the
gateway's `name` where the fleet file's `gateway` section gives one, or its
address otherwise (e.g. "Gateway (dev-2)" or "Gateway (localhost:4000)"), the
same "<label> (<qualifier>)" shape a remote environment already reads as (e.g.
"llama.cpp (dev-2)"). This keeps a second gateway from overwriting the first's
configured block, and — since the word a user actually searches a model picker
for is "gateway" — lets them find it at all.

#### Scenario: No Spinloop, a gateway routes anyway

- **WHEN** the user runs `spinloop code --fleet ./fleet.yaml` in a directory
  holding no Spinloop, and that fleet file names a gateway with its token set
  and models to list
- **THEN** the launch does not fail: the active harness is configured with a
  provider at the gateway's address, its model list populated from the
  gateway's `GET /v1/models`, no single default model set, and the harness
  launches against the gateway

#### Scenario: No Spinloop and no gateway still fails

- **WHEN** the user runs `spinloop code --fleet ./fleet.yaml` in a directory
  holding no Spinloop, and that fleet file names no gateway
- **THEN** the launch fails saying `--fleet` needs a Spinloop, and no harness
  is launched

#### Scenario: A named fleet file that cannot be read names itself

- **WHEN** the user runs `spinloop code --fleet ./fleet.yaml` where that file
  does not exist
- **THEN** the launch fails naming the fleet file it could not read, rather
  than the Spinloop it also lacks

#### Scenario: A Spinloop naming no model routes at a gateway

- **WHEN** a launch routes at a gateway with a Spinloop that names neither a
  model nor an alias
- **THEN** the launch does not fail on that account, and the harness's model
  list is populated from the gateway

#### Scenario: An unreachable gateway warns rather than failing

- **WHEN** a launch is configured against a gateway whose `GET /v1/models`
  cannot be completed
- **THEN** the launch warns, configures the harness with an empty model list,
  and still launches

#### Scenario: The configured provider is named for its gateway

- **WHEN** a launch is configured against a fleet file's gateway
- **THEN** the provider's display name leads with "Gateway" and is qualified by
  the section's `name` where it gives one, or by the gateway's address
  otherwise, so a second gateway does not overwrite the first's block
