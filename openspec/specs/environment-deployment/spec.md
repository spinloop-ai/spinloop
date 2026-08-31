# Environment Deployment Specification

## Purpose

Define how `spinloop remote deploy` creates a named environment on the
account-level control plane: discovering it, provisioning per-environment resources, registering the environment, and guarding against accidental overwrites.

## Requirements

### Requirement: Deploy creates an environment on the control plane

`spinloop remote deploy` SHALL create a named environment on top of the control
plane: it SHALL discover it, then provision the
environment's own Elastic IP, EC2 instance configuration, per-environment API
key, per-environment allowed-ingress rule, and per-environment SSM state
(the deploy-config), all tagged by the environment name. It SHALL set
what the environment serves from the Spinloop and its preset, and SHALL register
the environment so the other `remote` commands can drive it. Deploying SHALL NOT
start the instance.

Deploy SHALL NOT provision any activity-tracking state. Engine activity is
recorded on the instance by its daemon, so there is nothing for the control
plane to seed, read or write.

#### Scenario: Deploying stands up and registers an environment

- **WHEN** `spinloop remote deploy` runs for an environment against a bootstrapped
  account
- **THEN** the environment's Elastic IP, instance configuration, API key,
  ingress rule, and SSM state are provisioned, and the environment is registered

#### Scenario: Deploying provisions no activity state

- **WHEN** a deploy succeeds
- **THEN** no idle- or activity-tracking parameter is created for the
  environment

#### Scenario: Deploying is not starting

- **WHEN** a deploy succeeds
- **THEN** the environment is configured and registered but no instance is
  running until `spinloop remote start`
### Requirement: Discovering the control plane

Deploy SHALL discover the control plane from the bootstrap stack's
CloudFormation outputs (a well-known stack name) — the lifecycle Lambda URLs, the
weights bucket, the shared roles, and the region — rather than from any local
file. When the control-plane stack is absent, deploy SHALL fail telling the user to run
`spinloop remote bootstrap` first, rather than attempting to create an environment.

#### Scenario: The control plane is discovered

- **WHEN** deploy runs against a bootstrapped account
- **THEN** it reads the Lambda URLs, bucket, roles and region from the stack
  outputs

#### Scenario: Not bootstrapped

- **WHEN** deploy runs against an account with no control-plane stack
- **THEN** it fails saying to run `spinloop remote bootstrap` first, and creates
  nothing

### Requirement: Per-environment allowed ingress

Who may reach an environment's instance SHALL be scoped per environment. Deploy
SHALL take the allowed CIDR from a flag, defaulting to the caller's detected
public address as a `/32`, and apply it to that environment's security-group
rule. It SHALL NOT be an account-wide setting.

#### Scenario: Ingress defaults to the caller's address

- **WHEN** deploy runs without an explicit CIDR
- **THEN** the environment's ingress is the caller's public IP as a `/32`

#### Scenario: Ingress is per environment

- **WHEN** two environments are deployed with different CIDRs
- **THEN** each instance admits only its own CIDR

### Requirement: Registering the environment

Deploy SHALL register the environment in the per-user registry defined by the
Remote Environments specification — `~/.config/spinloop/remotes/<env>/remote.json`,
written owner-only — carrying the shared lifecycle Lambda URLs, the region, the
environment's base URL (its Elastic IP), and the environment identifier the
shared Lambdas use to select this environment's instance.

#### Scenario: The environment is registered and resolvable

- **WHEN** `spinloop remote deploy` for environment `prod` succeeds
- **THEN** `~/.config/spinloop/remotes/prod/remote.json` exists (owner-only) and an
  Spinloop stating `REMOTE prod` resolves to it

### Requirement: Refuse to overwrite a live environment

Before creating, deploy SHALL detect whether the target environment is already
registered or its instance is currently live. When it is, deploy SHALL show a
prominent warning and SHALL require explicit override consent — an `--overwrite`
flag, which `--yes` alone SHALL NOT satisfy — so a redeploy cannot silently
clobber a running instance. When neither holds, deploy SHALL proceed without a
warning and without requiring `--overwrite`.

#### Scenario: An existing environment warns

- **WHEN** deploy targets an environment that is already registered or live
- **THEN** it warns and does not overwrite unless `--overwrite` is given

#### Scenario: --yes does not imply override

- **WHEN** deploy is run with `--yes` but not `--overwrite` against an existing
  environment
- **THEN** it aborts telling the user to pass `--overwrite`

#### Scenario: A fresh environment needs no override

- **WHEN** deploy targets an environment that is neither registered nor live
- **THEN** it proceeds without an overwrite warning

### Requirement: Deploy accepts an optional spinloop version pin

`spinloop remote deploy` SHALL accept an optional flag pinning the exact spinloop
release the environment's instances install at boot. When the flag is given,
deploy SHALL record that version in the environment's stored deploy config so
the next fresh boot installs it; when it is absent, deploy SHALL record no pin
and the environment's boots install the latest published release (the boot's
default, defined by the `remote-engine-host` capability). An empty or
whitespace-only value SHALL be treated as if the flag were not given.

#### Scenario: A pin is recorded in the deploy config

- **WHEN** `spinloop remote deploy` runs with an spinloop version pin
- **THEN** the environment's stored deploy config carries that version, and
  the environment's next fresh boot installs exactly that release

#### Scenario: No pin leaves the boot on its default

- **WHEN** `spinloop remote deploy` runs without an spinloop version pin
- **THEN** the stored deploy config carries no spinloop version, and the
  environment's boots install the latest published release

#### Scenario: An empty pin value is ignored

- **WHEN** `spinloop remote deploy` is given an spinloop version pin whose value is
  empty or whitespace only
- **THEN** it is treated as if no pin were given

### Requirement: The deploy plan shows the resolved spinloop version

The plan `spinloop remote deploy` prints — including under `--dry-run`, before
any AWS work or send — SHALL state the spinloop version the environment's boots
will install: the pinned version when a pin is given, otherwise `latest`. It
SHALL appear alongside the runner and model the plan already prints.

#### Scenario: A pinned deploy prints the pinned version

- **WHEN** `spinloop remote deploy --dry-run` runs with an spinloop version pin
- **THEN** the printed plan names that pinned version as the spinloop the
  environment will run

#### Scenario: An unpinned deploy prints latest

- **WHEN** `spinloop remote deploy --dry-run` runs without an spinloop version pin
- **THEN** the printed plan names `latest` as the spinloop the environment will
  run
