## MODIFIED Requirements

### Requirement: Single resolved config directory

spinloop SHALL resolve one config directory and place every file it owns under
it: its own `config.json` (default-harness preference and alias registry), the
`remotes/<name>/` environment registry, the daemon state directory, and the CDK
source directory. There SHALL be one resolver; the location SHALL NOT be
computed independently in more than one place.

#### Scenario: All spinloop-owned state shares one root

- **WHEN** the config directory resolves to a given path
- **THEN** `config.json`, the `remotes/<name>/` registry, and the daemon state
  directory all resolve beneath that same path
