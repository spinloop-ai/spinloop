# Delta: environment-deployment

## MODIFIED Requirements

### Requirement: Deploy creates an environment on the control plane

`spinloop remote deploy` SHALL create a named environment on top of the control
plane: it SHALL discover it, then provision the
environment's own Elastic IP, EC2 instance configuration, per-environment API
key, per-environment allowed-ingress rule, and per-environment SSM state
(the deploy-config), all tagged by the environment name. It SHALL set
what the environment serves from the Spinloop and its preset, and SHALL register
the environment so the other `remote` commands can drive it. Deploying SHALL NOT
start the instance.

The environment name SHALL come from the command, never from the Spinloop:
`spinloop remote deploy` SHALL take it from its required `--env <name>` flag,
and `spinloop fleet deploy` SHALL take it from the name of the node being
deployed, which is the registered environment that node drives. A deploy given
no name by either route SHALL fail saying the environment must be named. The
Spinloop SHALL be read only for what the environment serves and for its local
environment (`ENV` instructions and adjacent `.env`), and the same Spinloop
SHALL be deployable under more than one environment name.

Deploy SHALL NOT provision any activity-tracking state. Engine activity is
recorded on the instance by its daemon, so there is nothing for the control
plane to seed, read or write.

#### Scenario: Deploying stands up and registers an environment

- **WHEN** `spinloop remote deploy --env prod` runs against a bootstrapped
  account
- **THEN** the environment's Elastic IP, instance configuration, API key,
  ingress rule, and SSM state are provisioned, and the environment is
  registered under the name `prod`

#### Scenario: A deploy without a name fails

- **WHEN** `spinloop remote deploy` runs with no `--env` flag
- **THEN** it fails saying the environment must be named with `--env <name>`

#### Scenario: One Spinloop deploys to two environments

- **WHEN** the same Spinloop is deployed with `--env dev` and later with
  `--env prod`
- **THEN** two distinct environments are created and registered, each serving
  what the Spinloop describes

#### Scenario: A fleet node deploys under its own name

- **WHEN** `spinloop fleet deploy` targets a `kind: remote` node named `qwen`
- **THEN** the environment created and registered is named `qwen`, from the
  node's name in the fleet file, and the node's Spinloop is read only for what
  it serves

#### Scenario: Deploying provisions no activity state

- **WHEN** a deploy succeeds
- **THEN** no idle- or activity-tracking parameter is created for the
  environment

#### Scenario: Deploying is not starting

- **WHEN** a deploy succeeds
- **THEN** the environment is configured and registered but no instance is
  running until `spinloop remote start`

### Requirement: Registering the environment

Deploy SHALL register the environment in the per-user registry defined by the
Remote Environments specification — `~/.config/spinloop/remotes/<env>/remote.json`,
written owner-only — carrying the shared lifecycle Lambda URLs, the region, the
environment's base URL (its Elastic IP), and the environment identifier the
shared Lambdas use to select this environment's instance.

#### Scenario: The environment is registered and resolvable

- **WHEN** `spinloop remote deploy --env prod` succeeds
- **THEN** `~/.config/spinloop/remotes/prod/remote.json` exists (owner-only)
  and `spinloop remote status --env prod` resolves the environment from it
