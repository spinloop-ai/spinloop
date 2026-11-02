# Spec Delta

## MODIFIED Requirements

### Requirement: Cloud environment storage names

Environments SHALL be stored under `~/.config/spinloop/clouds/<name>/cloud.json`
and credentials in `cloud-<region>.json` keystore files, with the same file
modes as before. The CLI SHALL NOT read or write `remotes/<name>/remote.json`
or `remote-<region>.json`.

#### Scenario: Writes use the new path
- **WHEN** `spinloop cloud deploy staging` finishes
- **THEN** `clouds/staging/cloud.json` exists with mode 0600

#### Scenario: Old environment is not found
- **WHEN** `remotes/staging/remote.json` exists and `clouds/staging/` does not
- **THEN** `spinloop status --node staging` reports that environment `staging` is not registered
