# Remote Environments Specification

## Purpose

Define the per-user registry of named remote environments: how a command's
`--env` value selects a deployment's control config by name, where that config
lives, how environments are listed, and the storage rules that keep per-user,
per-instance deployment state out of committed Spinloops.

## Requirements

### Requirement: Named environment registry

A remote environment SHALL be a directory under the per-user config,
`${XDG_CONFIG_HOME:-~/.config}/spinloop/remotes/<name>/`, whose canonical file is
`remote.json` — the control URLs, region, base URL, and the environment
identifier of one deployed instance. Because the lifecycle Lambda URLs are shared
across environments, the identifier is what selects this environment's instance;
the `remote` client SHALL send it with each control request. The directory form
SHALL be used so that other per-environment state may live alongside `remote.json`
later, and so that distinct environments never share a file. The registry SHALL
hold as many environments as the user has instances.

#### Scenario: An environment is a directory holding remote.json

- **WHEN** an environment named `qwen3.6-27b-prod` is registered
- **THEN** its configuration is `~/.config/spinloop/remotes/qwen3.6-27b-prod/remote.json`

#### Scenario: Two environments do not collide

- **WHEN** two environments `a` and `b` both exist
- **THEN** each has its own `~/.config/spinloop/remotes/<name>/` directory and
  neither overwrites the other

#### Scenario: The identifier selects the instance

- **WHEN** two environments share the same lifecycle Lambda URLs and a control
  command runs for one of them
- **THEN** the environment identifier in its `remote.json` is sent so the shared
  Lambda acts on that environment's instance

#### Scenario: A control call without an environment is rejected

- **WHEN** a control request reaches a lifecycle Lambda naming no environment
- **THEN** it is rejected with an error saying how to name one, rather than a
  default being silently assumed — defaults are a CLI affordance, not part of
  the control API

### Requirement: Environment name validity

An environment name SHALL be a plain name, not a path: it SHALL NOT contain a
path separator. A `--env` value that is not a plain identifier SHALL be
rejected saying an environment name is a plain identifier.

#### Scenario: A path-like value is not a name

- **WHEN** a command is given `--env ./remote.json` (or any value containing a
  path separator)
- **THEN** it fails saying an environment name is a plain identifier, rather
  than reading a file

### Requirement: Listing environments

`spinloop remote ls` SHALL print every registered environment with its base URL
and region, and SHALL mark an environment whose `remote.json` is missing or
unreadable rather than failing. With no environments registered it SHALL say so
plainly rather than printing nothing.

#### Scenario: Listing shows each environment

- **WHEN** two environments are registered and the user runs `spinloop remote ls`
- **THEN** both are listed with their base URL and region

#### Scenario: A missing configuration is marked, not fatal

- **WHEN** an environment directory exists without a readable `remote.json` and
  the user runs `spinloop remote ls`
- **THEN** that environment is listed with a missing/unreadable marker and the
  command still succeeds

#### Scenario: No environments registered

- **WHEN** the registry is empty and the user runs `spinloop remote ls`
- **THEN** the command says there are none, rather than printing empty output

### Requirement: Environment storage and isolation

Environment state SHALL live only under the per-user config directory, never in
a Spinloop, so Spinloops stay portable and committable — a Spinloop carries no
reference to a remote environment at all, not even a name. Each environment's
`remote.json` SHALL be written with owner-only permissions, since it holds a
deployment's URLs and address. Because state is per-user and keyed by name, two
users sharing a repo SHALL each drive their own instance under the same name
without either seeing the other's URLs.

#### Scenario: The Spinloop carries no environment reference

- **WHEN** a Spinloop used for a remote deployment is committed to a shared
  repo
- **THEN** it contains no environment name, deployment URL, or address; the
  environment is named only at deploy time and at the commands' invocations

#### Scenario: Owner-only configuration

- **WHEN** an environment's `remote.json` is written
- **THEN** it is created with owner-only permissions

### Requirement: An environment names the harness provider

When a Spinloop is applied with an `--env <name>` flag, the harness provider
SHALL be keyed on the environment name rather than the `PROVIDER` value, and
the default model SHALL read as `<environment>/<model>`. The `PROVIDER` entry
SHALL still supply the engine configuration (its options, API-key environment
variable, and base URL). The flag SHALL name a registered environment: a value
with no registered configuration SHALL fail saying the environment is not
registered and that `spinloop remote deploy --env <name>` creates it. A
Spinloop's own `BASEURL` SHALL still supply the applied base URL, as it does
when a fleet is named; the environment then provides the name and the key, not
the address. Unapplying with the same `--env` flag SHALL remove the provider
that apply wrote.

#### Scenario: The flag becomes the provider name

- **WHEN** a Spinloop stating `PROVIDER llamacpp` and `ALIAS qwen` is applied
  with `--env dev-1`
- **THEN** the harness config holds a provider keyed `dev-1` whose default
  model is `dev-1/qwen`, configured from the `llamacpp` catalogue entry

#### Scenario: A pinned BASEURL supplies the address

- **WHEN** a Spinloop stating a `BASEURL` is applied with `--env dev-1` whose
  registered configuration names a different base URL
- **THEN** the provider is keyed `dev-1` and configured with the Spinloop's
  `BASEURL`

#### Scenario: An unregistered environment is reported

- **WHEN** a Spinloop is applied with `--env missing` and no environment
  `missing` is registered
- **THEN** the command fails saying the environment is not registered and that
  `spinloop remote deploy --env missing` creates it

#### Scenario: Unapply removes the environment-named provider

- **WHEN** a Spinloop is applied with `--env dev-1` and then unapplied with
  `--env dev-1`
- **THEN** the provider keyed `dev-1` is removed from the harness config

### Requirement: Remote provider display name

When a Spinloop is applied with an `--env` flag, the harness provider's display
name SHALL be distinct from that of the local engine of the same kind, so the
remote environment and a local provider built from the same `PROVIDER` entry
are told apart in a harness model picker. The display name SHALL combine the
catalogue engine's display name with the environment name named by the flag
(for example `llama.cpp (dev-2)`); when the catalogue entry has no display
name, the environment name SHALL be used on its own. This labelling SHALL apply
only to the display name — the provider key, the `<environment>/<model>`
default model, the engine options, the API-key environment variable, and the
base URL are unchanged from the existing remote-naming behaviour.

#### Scenario: A remote provider is labelled with its environment

- **WHEN** a Spinloop stating `PROVIDER llamacpp` (display name `llama.cpp`)
  and `ALIAS qwen` is applied with `--env dev-2`
- **THEN** the harness config holds a provider keyed `dev-2` whose display name
  is `llama.cpp (dev-2)`, distinct from a local `llamacpp` provider's
  `llama.cpp`

#### Scenario: A local and a remote engine of the same kind are distinguishable

- **WHEN** both a local `llamacpp` provider and a remote `dev-2` provider built
  from the same engine are present in the harness config
- **THEN** their display names differ, so the two appear as separate rows in
  the harness model picker rather than two identical `llama.cpp` rows

#### Scenario: An engine with no display name falls back to the environment name

- **WHEN** a Spinloop's `PROVIDER` catalogue entry has no display name and the
  Spinloop is applied with `--env dev-2`
- **THEN** the harness provider's display name is `dev-2`
