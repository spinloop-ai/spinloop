# Spec Delta

## RENAMED Requirements

- FROM: `### Requirement: Cloud environment storage names`
- TO: `### Requirement: Cloud node names`
## MODIFIED Requirements

### Requirement: Cloud node names

Control-plane credentials SHALL be stored in `cloud-<region>.json` keystore
files where the file store is in use (and in the OS keystore where one is
reachable), with the same owner-only file modes as before. Nodes SHALL be
stored nowhere: there is no `clouds/<name>/cloud.json` to write, read, or
migrate, because the plane is the record of what is deployed. The CLI SHALL NOT
read or write `remotes/<name>/remote.json` or `remote-<region>.json`, and SHALL
NOT read a `clouds/<name>/cloud.json` left behind by an earlier version.

#### Scenario: Writes use the new path

- **WHEN** `spinloop cloud auth --store` stores a key for `eu-west-1` on a
  machine with no reachable OS keystore
- **THEN** `cloud-eu-west-1.json` holds it with mode 0600

#### Scenario: A deploy writes no node file

- **WHEN** `spinloop cloud deploy --node staging` finishes
- **THEN** nothing beneath the configuration directory names `staging`, and
  `spinloop cloud ls` lists it from the plane

#### Scenario: Old environment is not found

- **WHEN** `remotes/staging/remote.json` or `clouds/staging/cloud.json` exists
  from an earlier version, and the plane holds no node `staging`
- **THEN** `spinloop status --node staging` reports that the plane holds no such
  node, having read neither file

### Requirement: Cloud environment variables

The CLI SHALL read its cloud settings from `SPINLOOP_CLOUD_*` variables
(`SPINLOOP_CLOUD_REGION`, `_START_URL`, `_STOP_URL`, `_STATS_URL`,
`_DEPLOY_URL`, `_ENV_URL`, `_SEED_URL`, `_UPDATE_URL`, `_KEYSTORE`,
`_PACKAGE_MANAGER`). They SHALL configure the resolved plane — overriding
individual values, or stating a plane outright where enough of them are set —
rather than filling in a stored configuration file. `SPINLOOP_REMOTE_*`
variables SHALL be ignored.

#### Scenario: New variable is read

- **WHEN** `SPINLOOP_CLOUD_REGION=us-east-1` is set
- **THEN** the plane searched is the stack in `us-east-1`

#### Scenario: A stated endpoint overrides the discovered plane

- **WHEN** `SPINLOOP_CLOUD_START_URL` is set and a control-plane stack is also
  visible
- **THEN** the start endpoint used is the stated one

#### Scenario: Old variable is ignored

- **WHEN** only `SPINLOOP_REMOTE_REGION=eu-west-2` is set
- **THEN** the region is treated as unset

### Requirement: The cloud fleet node kind

A fleet file node SHALL accept `kind: cloud` for a cloud node deployed on
the plane the caller's credentials and region resolve. `kind: remote` SHALL be
rejected with the same error as any other unknown kind. Fleet output, status and
dashboard SHALL label such nodes `cloud`.

#### Scenario: New kind parses

- **WHEN** a fleet file declares a node with `kind: cloud`
- **THEN** it is loaded as a cloud node node, keyed by its name on the
  resolved plane

#### Scenario: Old kind is rejected

- **WHEN** a fleet file declares a node with `kind: remote`
- **THEN** loading fails with an unknown-kind error naming the node
