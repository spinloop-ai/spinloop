# Delta: remote-environments

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Environment name validity

An environment name SHALL be a plain name, not a path: it SHALL NOT contain a
path separator. A `--env` value that is not a plain identifier SHALL be
rejected saying an environment name is a plain identifier.

#### Scenario: A path-like value is not a name

- **WHEN** a command is given `--env ./remote.json` (or any value containing a
  path separator)
- **THEN** it fails saying an environment name is a plain identifier, rather
  than reading a file

## REMOVED Requirements

### Requirement: Resolving a REMOTE value to an environment or a file

**Reason**: The `REMOTE` instruction is removed from the Spinloop grammar, and
with it the path/URL form of a remote configuration. An environment is selected
by name at the command line.

**Migration**: Pass `--env <name>` to the `remote` subcommands, `apply`,
`unapply`, and `harness`; the configuration is read from
`~/.config/spinloop/remotes/<name>/remote.json`. A path- or URL-form
configuration is no longer addressable; the `SPINLOOP_REMOTE_*` overrides carry
a manual configuration where one is needed.

### Requirement: A REMOTE names the harness provider

**Reason**: The `REMOTE` instruction is removed from the Spinloop grammar; the
environment name now arrives as the `--env` flag of the command.

**Migration**: Apply and unapply with `--env <name>`; the provider is keyed on
the flagged environment exactly as a `REMOTE`-named one was.

### Requirement: A remote harness provider is labelled distinctly

**Reason**: The path-form `REMOTE` (and its "no environment in the config"
fallback scenario) is removed; the environment name now arrives as the `--env`
flag, which always names an environment.

**Migration**: Apply with `--env <name>`; the labelling is unchanged, keyed on
the flagged environment.

### Requirement: Registry storage and isolation

**Reason**: A Spinloop no longer carries even an environment name, so the
"carries only the name" framing is obsolete; the storage and permission rules
are unchanged.

**Migration**: None — the registry layout, permissions, and per-user isolation
are unchanged.
