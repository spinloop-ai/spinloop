# Spec Delta

## MODIFIED Requirements

### Requirement: Reachability warning message

When the probe fails, the warning SHALL state that the endpoint is ready but not reachable from the current network, and SHALL provide a remediation command using `spinloop cloud deploy --overwrite --allowed-cidr`. The command SHALL include the caller's public IP as a /32 CIDR, detected by calling `checkip.amazonaws.com`. If IP detection fails, the warning SHALL use a placeholder value instead of failing the command.

The warning SHALL NOT cause the command to exit with a non-zero status. The command SHALL still print exports (if `--print-env` is set) and exit 0.

#### Scenario: Warning includes the caller's IP

- **WHEN** the probe fails and the IP detection succeeds
- **THEN** the remediation command includes the caller's public IP as `/32`

#### Scenario: IP detection failure does not break start

- **WHEN** the probe fails and the IP detection also fails
- **THEN** the warning is still printed with a placeholder CIDR and the command exits 0
