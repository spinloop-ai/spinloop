# Spec Delta

## REMOVED Requirements

### Requirement: Named environment registry

**Reason**: The per-user directory of registered environments was a local cache
of facts the control plane already holds — each environment's deploy config,
address, key and instance — and being local it was wrong for everyone but the
machine that wrote it. The plane is now the record; see the new
`cloud-plane-resolution` and `cloud-environment-inventory` capabilities.

**Migration**: Nothing to migrate: the plane already holds what each
`cloud.json` held. Deleting `~/.config/spinloop/clouds/` changes nothing.
`spinloop cloud ls` now lists the plane's environments; `--node <name>` now
resolves a name against the plane the caller's credentials and region resolve.

### Requirement: Environment storage and isolation

**Reason**: It required environment state to live under the per-user
configuration directory, which is exactly the state being removed. Its two
lasting guarantees move elsewhere: a Spinloop still carries no reference to a
cloud environment (the `spinloop-files` specification), and a file that may hold
a secret is still written owner-only (the `config-location` specification).
Per-user isolation of same-named environments is deliberately dropped:
environments on a plane are shared, and their names are plane-global.

**Migration**: Two people sharing a plane now share one name for one
environment, which is what a shared deployment means. To keep private
environments, deploy them in an account of one's own — the credential selects
the plane — or give them distinct names.

### Requirement: Every environment resolves by name

**Reason**: It resolved a name by reading `clouds/<name>/cloud.json` under the
per-user registry, and every one of its scenarios was about that file. Names now
resolve against the plane the caller's credentials and region resolve; the
replacement requirement is `An environment resolves by name on the plane`.

**Migration**: Nothing to do for names that resolve today. A name that only ever
existed as a local file — hand-written, or deployed against a plane that no
longer exists — resolves to nothing, and the failure names the plane to check.

## MODIFIED Requirements

### Requirement: Environment name validity

An environment name SHALL be a plain name, not a path: it SHALL NOT contain a
path separator, and it SHALL satisfy the shape the control plane accepts for an
environment name. A `--node` value that is not a plain identifier SHALL be
rejected saying an environment name is a plain identifier.

Because a name identifies one environment on the plane the caller's credentials
resolve, the same name on two planes SHALL be two environments, and the same
name on one plane SHALL be one environment however many people use it.

#### Scenario: A path-like value is not a name

- **WHEN** a command is given `--node ./cloud.json` (or any value containing a
  path separator)
- **THEN** it fails saying an environment name is a plain identifier, rather
  than reading a file or querying a plane

#### Scenario: A name the control plane would refuse is refused early

- **WHEN** a command is given a name the control plane would reject
- **THEN** it fails before contacting the plane, naming the shape a name must
  have

#### Scenario: The same name on two planes is two environments

- **WHEN** two operators each run `--node dev` with credentials for different
  accounts
- **THEN** each acts on the `dev` deployed in their own account

### Requirement: Listing environments

`spinloop cloud ls` SHALL print every environment deployed on the resolved
plane with its deployer, what it serves, its address and its state, rather than
what this machine has recorded. With no environments on the plane it SHALL say
so plainly rather than printing nothing. Where the plane cannot be resolved or
reached it SHALL fail naming where it looked rather than printing a partial
list.

#### Scenario: Listing shows each environment

- **WHEN** two environments are deployed on the plane and the user runs
  `spinloop cloud ls`
- **THEN** both are listed with their deployer, what they serve, their address
  and their state

#### Scenario: An environment deployed elsewhere is listed

- **WHEN** an environment deployed by another operator is on the plane and the
  user runs `spinloop cloud ls`
- **THEN** it is listed like any other, credited to its deployer

#### Scenario: A missing configuration is marked, not fatal

- **WHEN** an environment is deployed but the plane cannot report its state or
  address — its instance was terminated mid-read, or its daemon is unreachable
- **THEN** that environment is listed with a marker where the readings are
  missing, and the command still succeeds

#### Scenario: No environments registered

- **WHEN** the plane holds no environments and the user runs `spinloop cloud ls`
- **THEN** the command says there are none, rather than printing empty output

#### Scenario: An unreadable plane is not an empty one

- **WHEN** `spinloop cloud ls` runs where the plane cannot be reached
- **THEN** the command fails naming where it looked, and prints no list

### Requirement: An environment names the harness provider

When a Spinloop is applied with an `--node <name>` flag, the harness provider
SHALL be keyed on the environment name rather than the `PROVIDER` value, and
the default model SHALL read as `<environment>/<model>`. The `PROVIDER` entry
SHALL still supply the engine configuration (its options, API-key environment
variable, and base URL). The flag SHALL name an environment deployed on the
resolved plane: a value with no such environment SHALL fail saying the plane
holds no environment of that name and that `spinloop cloud deploy --node <name>`
creates one. A Spinloop's own `BASEURL` SHALL still supply the applied base
URL, as it does when a fleet is named; where the Spinloop states none, the base
URL SHALL be the environment's own address as the plane reports it, without
starting the environment. Unapplying with the same `--node` flag SHALL remove
the provider that apply wrote.

#### Scenario: The flag becomes the provider name

- **WHEN** a Spinloop stating `PROVIDER llamacpp` and `ALIAS qwen` is applied
  with `--node dev-1`
- **THEN** the harness config holds a provider keyed `dev-1` whose default
  model is `dev-1/qwen`, configured from the `llamacpp` catalogue entry

#### Scenario: A pinned BASEURL supplies the address

- **WHEN** a Spinloop stating a `BASEURL` is applied with `--node dev-1` whose
  environment answers at a different address
- **THEN** the provider is keyed `dev-1` and configured with the Spinloop's
  `BASEURL`

#### Scenario: No BASEURL takes the environment's address from the plane

- **WHEN** a Spinloop stating no `BASEURL` is applied with `--node dev-1`
- **THEN** the provider is configured with the address the plane reports for
  `dev-1`, and the environment is not started to learn it

#### Scenario: An unregistered environment is reported

- **WHEN** a Spinloop is applied with `--node missing` and the plane holds no
  environment `missing`
- **THEN** the command fails saying the plane holds no such environment and that
  `spinloop cloud deploy --node missing` creates it

#### Scenario: Unapply removes the environment-named provider

- **WHEN** a Spinloop is applied with `--node dev-1` and then unapplied with
  `--node dev-1`
- **THEN** the provider keyed `dev-1` is removed from the harness config

## ADDED Requirements

### Requirement: An environment resolves by name on the plane

An environment SHALL be resolved by its name and nothing else: the environment
of that name on the plane the caller's credentials and region resolve. No name
SHALL resolve by a different rule from any other, and no local file SHALL be
consulted for any name, so a command that resolves an environment never
special-cases the name it was given and never depends on what this machine has
deployed.

Where the caller's variables supply a complete set of control endpoints, those
SHALL be the plane, so the cloud commands work in a place with no
CloudFormation-visible stack. In that case the **name given** SHALL be the
environment identifier sent with each control call — the identifier the shared
lifecycle endpoints use to tell one instance from another.

#### Scenario: A named environment is found on the plane

- **WHEN** a command runs with `--node prod` and the resolved plane holds an
  environment `prod`
- **THEN** the command drives that environment, whether or not this machine ever
  deployed it

#### Scenario: Nothing local is consulted

- **WHEN** a command runs with `--node prod` and a file for an environment
  `prod` exists under the configuration directory from an earlier version
- **THEN** the command uses the plane's answer and does not read that file

#### Scenario: Overrides configure a plane with no stack

- **WHEN** a command runs with `--node ci`, no control-plane stack is visible,
  and the `SPINLOOP_CLOUD_*` variables supply a complete set of control
  endpoints
- **THEN** the command works, and the control calls carry `ci` as the
  environment identifier

#### Scenario: An environment the plane does not hold fails naming the plane

- **WHEN** no environment of the given name is deployed on the plane that
  resolves
- **THEN** the failure says the plane holds no such environment, names the
  account and region resolved, and says which command creates one
