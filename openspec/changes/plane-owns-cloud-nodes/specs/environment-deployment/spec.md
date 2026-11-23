# Spec Delta

## REMOVED Requirements

### Requirement: Registering the environment

**Reason**: Deploy wrote the plane's control URLs, the region, the environment's
address and its own name into a per-user file so later commands could find it.
The plane holds all of that already, and the resolution of a plane from
credentials and region is defined by the new `cloud-plane-resolution`
capability, so there is nothing left for a deploy to record locally.

**Migration**: Deploy writes nothing. Later commands resolve the plane from the
caller's credentials and region and the node by name, so a node
deployed by anyone is drivable by anyone with credentials for that plane. A
file left behind by an earlier version is ignored and may be deleted.

## RENAMED Requirements

- FROM: `### Requirement: Per-environment allowed ingress`
- TO: `### Requirement: Per-node allowed ingress`
- FROM: `### Requirement: Refuse to overwrite a live environment`
- TO: `### Requirement: Refuse to overwrite a live node`
## MODIFIED Requirements

### Requirement: Discovering the control plane

Deploy — and every other command that speaks to the control plane — SHALL
discover it from the bootstrap stack's CloudFormation outputs (a well-known
stack name), read for the region the caller's credentials resolve: the
lifecycle endpoint URLs, the weights bucket, the shared roles, and the region.
No command SHALL read a control URL from a local file, and the answer SHALL be
held in memory for the process rather than written to disk. When the
control-plane stack is absent, a command SHALL fail telling the user to run
`spinloop cloud bootstrap` first, naming the account and region searched,
rather than attempting to create a node.

#### Scenario: The control plane is discovered

- **WHEN** deploy runs against a bootstrapped account
- **THEN** it reads the endpoint URLs, bucket, roles and region from the stack
  outputs

#### Scenario: Every command discovers rather than reading a file

- **WHEN** `spinloop cloud status`, `stats`, `logs` or `undeploy` runs
- **THEN** each resolves the control URLs from the stack outputs, and none reads
  a stored configuration

#### Scenario: Not bootstrapped

- **WHEN** a cloud command runs against an account with no control-plane stack
- **THEN** it fails saying to run `spinloop cloud bootstrap` first, naming the
  account and region searched, and creates nothing

### Requirement: Per-node allowed ingress

Who may reach a node's instance SHALL be scoped per node. Deploy
SHALL take the allowed CIDR from a flag and apply it to that node's
security-group rule. It SHALL NOT be an account-wide setting.

Where no CIDR is stated, the default SHALL depend on whether the node is
already deployed on the plane, as the plane reports it — not on whether this
machine has a record of it. A **new** node SHALL default to the caller's
detected public address as a `/32`. An **existing** node SHALL NOT be
defaulted: widening a shared node's ingress to whichever colleague is
deploying this minute is not a safe default, so deploy SHALL refuse, naming
`--allowed-cidr` as the fix and leaving the node's existing ingress
alone.

#### Scenario: Ingress defaults to the caller's address

- **WHEN** deploy creates a node that the plane does not already hold,
  with no explicit CIDR
- **THEN** the node's ingress is the caller's public IP as a `/32`

#### Scenario: Ingress is per environment

- **WHEN** two nodes are deployed with different CIDRs
- **THEN** each instance admits only its own CIDR

#### Scenario: An existing node without a stated CIDR is refused

- **WHEN** deploy targets a node the plane already holds and states no
  CIDR
- **THEN** it refuses naming `--allowed-cidr`, leaves the node's existing
  ingress unchanged, and deploys nothing

#### Scenario: A stated CIDR is applied to the existing node

- **WHEN** deploy targets an existing node with `--overwrite` and an
  explicit CIDR
- **THEN** that node's ingress becomes the stated CIDR and no address is
  detected from the caller

### Requirement: Refuse to overwrite a live node

Before creating, deploy SHALL ask the plane whether the target node is
already deployed or its instance currently live — not whether this machine
remembers it. When either holds, deploy SHALL show a prominent warning naming
who deployed it, when, and what it serves, and SHALL require explicit override
consent — an `--overwrite` flag, which `--yes` alone SHALL NOT satisfy — so a
redeploy cannot silently clobber a colleague's running instance. When neither
holds, deploy SHALL proceed without a warning and without requiring
`--overwrite`.

#### Scenario: An existing environment warns

- **WHEN** deploy targets a node already deployed on the plane by
  another operator
- **THEN** it warns, naming that operator, the deploy time and what is served,
  and does not overwrite unless `--overwrite` is given

#### Scenario: --yes does not imply override

- **WHEN** deploy is run with `--yes` but not `--overwrite` against an existing
  node
- **THEN** it aborts telling the user to pass `--overwrite`

#### Scenario: A fresh environment needs no override

- **WHEN** deploy targets a node the plane neither holds nor has running
- **THEN** it proceeds without an overwrite warning

#### Scenario: A colleague's deployment is seen from a clean machine

- **WHEN** deploy targets a node deployed by someone else, from a
  machine that has never held any cloud configuration
- **THEN** the guard still fires, because it asks the plane
