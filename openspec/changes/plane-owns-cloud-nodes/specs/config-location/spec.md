# Spec Delta

## MODIFIED Requirements

### Requirement: Single resolved config directory

spinloop SHALL resolve one config directory and place every file it owns under
it: its own `config.json` (default-harness preference and alias registry), the
stored control-plane credentials, the daemon state directory, and the CDK source
directory. There SHALL be one resolver; the location SHALL NOT be computed
independently in more than one place.

What lives there SHALL be only state that is genuinely this machine's — a secret
the caller stored, a build cache, or a daemon's own working state. No cloud
node, control endpoint, plane address, or other fact about the cloud SHALL
be recorded there, because the cloud is the record of the cloud.

#### Scenario: All spinloop-owned state shares one root

- **WHEN** the config directory resolves to a given path
- **THEN** `config.json`, the stored credentials, the daemon state directory and
  the CDK source directory all resolve beneath that same path

#### Scenario: No cloud fact is stored

- **WHEN** cloud nodes are deployed, listed, started and stopped from a
  machine
- **THEN** nothing beneath the config directory names a node, a control
  endpoint or a node address, other than a credential the caller
  explicitly stored
