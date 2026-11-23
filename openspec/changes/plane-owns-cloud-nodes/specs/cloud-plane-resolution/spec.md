# Spec Delta

## Purpose

Decides which cloud control plane a command speaks to from the caller's
credentials and region alone, and what spinloop refuses to keep on disk about
the cloud.

## ADDED Requirements

### Requirement: The plane is resolved from credentials and region

A command that speaks to a cloud control plane SHALL resolve that plane from
the caller's AWS credentials and region and nothing else: the well-known
control-plane stack in the resolved region, read from its CloudFormation
outputs, which name the control endpoints, the region, and the weights bucket.
No local file SHALL name a plane, and no stored configuration SHALL be
consulted before the stack is read, so the same command run by two people with
credentials for the same plane resolves the same plane.

The region searched SHALL be the one the standard AWS region resolution yields
(an explicit override, then the profile's region). Where no region resolves,
the command SHALL fail naming the variables that would say which plane to look
in, and contact nothing.

#### Scenario: Credentials and region pick the plane

- **WHEN** `spinloop cloud status --node prod` runs with credentials for one
  account and a region of `eu-west-1`
- **THEN** the control endpoints come from that account's `cloud-vm-llm` stack
  outputs in `eu-west-1`, and the node `prod` is read there

#### Scenario: No local record is read or required

- **WHEN** a cloud command runs on a machine that has never deployed an
  node and holds no cloud configuration of its own
- **THEN** the command resolves the plane and works, writing nothing

#### Scenario: Two people resolve one plane

- **WHEN** two operators with credentials for the same account and region each
  run `spinloop cloud status --node prod`
- **THEN** both speak to the same control plane and report the same node

#### Scenario: No region resolves

- **WHEN** neither an explicit region nor a profile's region can be determined
- **THEN** the command fails naming the region variables that would say which
  plane to look in, and contacts nothing

### Requirement: A resolved plane is remembered for the process

A process SHALL resolve a plane at most once for the calls it makes, holding the
answer in memory only. A resolved plane SHALL NOT be written to disk, cached
between runs, or given an expiry: it is a resolution, not a record.

A control call rejected as unauthorised, or failing because its host cannot be
reached, SHALL cause the plane to be resolved once more and the call retried
once before the failure is reported — so a plane redeployed while a long-lived
process was running heals at its first failed call rather than staying wrong.

#### Scenario: One resolution serves a whole fan-out

- **WHEN** a command reads four cloud nodes in one fleet
- **THEN** the plane is resolved once and all four calls use that answer

#### Scenario: A long-lived process resolves once

- **WHEN** a dashboard refreshes a cloud node's status every few seconds
  for an hour
- **THEN** the plane is resolved on the first refresh and not polled again while
  the process runs

#### Scenario: A rejected call re-resolves once

- **WHEN** a control call is rejected because its endpoint no longer exists
  after a plane redeploy
- **THEN** the plane is resolved again, the call is retried once against the new
  endpoints, and the caller sees either a result or one clear failure

#### Scenario: Nothing is written to disk

- **WHEN** any cloud command finishes
- **THEN** no file beneath spinloop's configuration directory holds a control
  endpoint, a plane address, or a node name

### Requirement: Cloud environment variables configure a plane directly

The `SPINLOOP_CLOUD_*` variables SHALL configure a plane outright, taking
precedence over a discovered one value at a time, so a caller can drive a plane
that is not bootstrapped at the well-known stack name — or state endpoints for
a machine with no CloudFormation permission. A set of variables naming a
complete set of control endpoints SHALL be enough to work with nothing else,
and an incomplete set SHALL fail naming the missing values.

#### Scenario: Overrides supply the endpoints

- **WHEN** the control endpoint variables are set and a command runs
- **THEN** those endpoints are used, whether or not a control-plane stack
  exists

#### Scenario: Overrides beat a discovered plane

- **WHEN** a bootstrapped plane exists and one endpoint variable is also set
- **THEN** that endpoint comes from the variable and the rest from the stack

#### Scenario: Partial overrides name what is missing

- **WHEN** the variables name some but not all of the endpoints no command can
  work without
- **THEN** the command fails naming the endpoints still missing

### Requirement: An unresolvable plane names where it looked

Where no plane resolves, the failure SHALL say what was looked for and where,
and what would make it resolve: the stack name searched, the account and region
the credential belongs to, and the fix — bootstrap the plane, choose another
region, or use credentials for the account that holds it. It SHALL NOT report a
node as missing when the plane holding it could not be reached.

#### Scenario: The account is not bootstrapped

- **WHEN** a cloud command runs against an account and region with no
  control-plane stack
- **THEN** it fails naming the stack searched, the account and region, and
  `spinloop cloud bootstrap` as the fix, creating nothing

#### Scenario: The credential is rejected

- **WHEN** the caller's credentials are expired or invalid
- **THEN** the failure says the credential was rejected and how to refresh it,
  rather than reporting that no nodes exist

### Requirement: spinloop keeps no cloud inventory locally

spinloop SHALL NOT record locally which cloud nodes exist, which plane
they belong to, or what address any of them answers on. Anything it owns under
its configuration directory SHALL be either a secret the caller stored or a
build cache — never a fact about the cloud.

#### Scenario: A fresh machine sees everything

- **WHEN** `spinloop cloud ls` runs on a machine that has never deployed
  anything, with credentials for a plane holding three nodes
- **THEN** all three are listed

#### Scenario: A shared deployment needs no local copy

- **WHEN** an operator drives a node a colleague deployed
- **THEN** no file is created for it on the operator's machine, and every later
  command resolves it from the plane
