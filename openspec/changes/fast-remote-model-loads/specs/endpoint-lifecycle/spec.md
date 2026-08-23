## MODIFIED Requirements

### Requirement: Starting on demand

Each environment SHALL hold no running instance when idle. A start request names
an environment and SHALL launch that environment's instance, trying each
configured availability zone in turn until one has capacity, since GPU capacity
is not guaranteed in any single zone. A launch SHALL provision the instance's
root volume: a gp3 volume of the AMI's own root size, with provisioned
throughput at the volume's ceiling — the size is read from the AMI's own root
mapping, because a launch's block device mapping replaces the AMI's rather
than extending it — and IOPS SHALL remain at the volume's baseline. The
instance SHALL be given the environment's own stable address (its Elastic IP)
so the environment's URL does not change between launches, and the request
SHALL NOT report success until the model is answering — the caller receives
one "ready", never a URL that is not yet serving. When no capacity can be
found anywhere, the response SHALL say so and SHALL be retryable rather than
fatal. One shared set of lifecycle Lambdas SHALL serve every environment in
the account, selecting the instance by the environment identifier.

The control plane SHALL request the engine's start on every path — a fresh
launch and a re-wake alike — once the instance's daemon answers its control
API, which on a fresh boot is the signal that the boot has stored the deploy
config; the boot's own user data SHALL NOT start the engine. The start SHALL
carry the deploy config as its body, so it always names the exact config the
daemon runs. A start request MAY carry a pre-warm choice for the engine's
start, which SHALL ride in that body; absent one, the cloud default (the
pre-warm enabled) SHALL apply.

#### Scenario: A zone without capacity is not the end of it

- **WHEN** the first availability zone cannot provide the instance type
- **THEN** the remaining zones are tried before reporting failure

#### Scenario: Ready means serving

- **WHEN** a start request returns success
- **THEN** the model is answering requests at the environment's reported address

#### Scenario: No capacity anywhere

- **WHEN** every configured zone is out of capacity
- **THEN** the response says so and indicates the caller may retry shortly

#### Scenario: Starting the right environment

- **WHEN** several environments are deployed and a start names one of them
- **THEN** only that environment's instance is launched, at its own Elastic IP

#### Scenario: Nothing has been deployed

- **WHEN** a start is requested for an environment before it has been deployed
- **THEN** it fails saying what to deploy, rather than launching an instance
  with nothing to serve

#### Scenario: A launch provisions the root volume

- **WHEN** a start launches a fresh instance
- **THEN** its root volume is the AMI's gp3 root, at the AMI's own size, with
  provisioned throughput at the volume's ceiling and IOPS at the baseline

#### Scenario: The control plane starts the engine on a fresh boot

- **WHEN** a fresh instance's daemon first answers its control API
- **THEN** the start request itself issues the engine's start, with the
  deploy config as its body, and reports ready only once the model answers —
  the boot started no engine

#### Scenario: A start may carry the pre-warm choice

- **WHEN** a start request carries the pre-warm choice disabled
- **THEN** the engine's start for that wake pre-warms no page cache, and a
  start that carries none pre-warms as the cloud default says
