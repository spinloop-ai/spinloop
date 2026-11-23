# Spec Delta

## REMOVED Requirements

### Requirement: Environment selection for cloud commands

**Reason**: The requirement selected an environment by reading its
`cloud.json` from the per-user registry, and its scenarios both demanded and
forbade a `default` fallback. The control URLs now come from the plane the
caller's credentials and region resolve, and there is definitively no fallback.
The replacement requirement is `Environments are selected by name on the
resolved plane`.

**Migration**: `--node <name>` is unchanged at the keyboard; what changes is
where the name is looked up. Scripts that relied on an assumed `default`
environment must name it.

## ADDED Requirements

### Requirement: Environments are selected by name on the resolved plane

The endpoint's control URLs SHALL come from the control plane the caller's
credentials and region resolve, read from its stack outputs, plus the
`SPINLOOP_CLOUD_*` overrides where those state values directly; they SHALL NOT
come from a stored configuration file. A resolved plane names a start URL, a
stop URL, and the other control endpoints, and a region. The endpoint's own base
URL SHALL NOT need to be held anywhere: the control calls report the address
themselves.

A `cloud` subcommand SHALL select which environment it acts on with its
`--node <name>` flag, and the flag SHALL be required: the value is an
environment name on the resolved plane. A `--node` value naming an environment
the plane does not hold, and which the variables do not configure, SHALL fail
saying the plane holds no such environment and how to create it, naming the
account and region resolved.

A subcommand given no `--node` SHALL fail naming the flag and listing the
plane's environments, and SHALL act on nothing. There is no environment a
command falls back to: several of these subcommands change the state of a cloud
instance, an instance nobody named is not one to start, stop or terminate, and
on a shared plane an assumed name could act on a colleague's deployment. The
name `default` is an ordinary environment name, carrying no special meaning.

The Spinloop a subcommand is given as an argument SHALL NOT select an
environment; it SHALL be read only for its `ENV` instructions and the `.env`
file beside it, which the command applies before any AWS or control-plane work
(see the Cloud Local Environment specification).

Environment variables SHALL override individual values, and the region SHALL
fall back to the standard AWS region variables. A plane that cannot be resolved
SHALL fail naming where it looked.

#### Scenario: The flag selects the environment

- **WHEN** the user runs `spinloop cloud status --node qwen3.6-27b-prod`
- **THEN** the control URLs come from the resolved plane and the environment is
  the one named, wherever it was deployed from

#### Scenario: An environment the plane lacks is named as such

- **WHEN** a `cloud` subcommand runs with `--node missing` and the resolved
  plane holds no environment `missing`
- **THEN** it fails saying the plane holds no such environment, naming the
  account and region resolved and that `spinloop cloud deploy --node missing`
  creates it

#### Scenario: No flag acts on nothing

- **WHEN** a `cloud` subcommand runs with no `--node` flag
- **THEN** it fails naming the flag and listing the plane's environments, and
  contacts no environment

#### Scenario: An explicit Spinloop does not select an environment

- **WHEN** a `cloud` subcommand is given a Spinloop as its argument and no
  `--node` flag
- **THEN** the command fails naming `--node`, having applied the Spinloop's
  `ENV` instructions and adjacent `.env` to the process environment, rather than
  assuming an environment for it

#### Scenario: No address needs to be held anywhere

- **WHEN** a `cloud` subcommand runs against an environment whose address has
  changed since it was deployed, and nothing local names any address
- **THEN** the subcommand works, because the endpoint reports its own address in
  the replies to `start` and `status`

#### Scenario: An instance is never stopped without being named

- **WHEN** the user runs `spinloop cloud stop` with no `--node`, with an
  environment named `default` deployed on the plane
- **THEN** nothing is stopped: the command fails naming the flag, and `default`
  is not assumed

#### Scenario: default is an ordinary name

- **WHEN** the user runs a `cloud` subcommand with `--node default` and the plane
  holds an environment `default`
- **THEN** it acts on that environment, exactly as it would for any other name
