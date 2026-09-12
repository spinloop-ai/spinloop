## MODIFIED Requirements

### Requirement: Starting on demand

Each environment SHALL hold no running instance when idle. A start request names
an environment and SHALL launch that environment's instance, trying each
configured availability zone in turn until one has capacity, since GPU capacity
is not guaranteed in any single zone. A launch SHALL provision the instance's
root volume: a gp3 volume of the AMI's own root size, with provisioned
throughput at the volume's ceiling — the size is read from the AMI's own root
mapping, because a launch's block device mapping replaces the AMI's rather
than extending it — and IOPS provisioned at four times that throughput,
which is the minimum EC2 allows for it (gp3 caps throughput at 0.25 MiB/s
per provisioned IOP). The
instance SHALL be given the environment's own stable address (its Elastic IP)
so the environment's URL does not change between launches, and the request
SHALL NOT report success until the model is answering — the caller receives
one "ready", never a URL that is not yet serving. When no capacity can be
found anywhere, the response SHALL say so, SHALL name the instance type it was
trying to launch, and SHALL be retryable rather than
fatal. One shared set of lifecycle Lambdas SHALL serve every environment in
the account, selecting the instance by the environment identifier.

A launch SHALL use the instance type the environment's deploy config names,
when it names one, and the control plane's default type otherwise. The type is
a property of the environment's deployment, read from the same stored deploy
config the start already reads for what to serve, so changing it is a deploy,
not a start. Because EC2 cannot change the type of an existing instance, a
stored type takes effect on a fresh launch only: a re-wake of a stopped
instance SHALL keep the type it was originally launched with, and a changed
type applies once the instance has been terminated — by an explicit stop or
the idle sweep — and relaunched.

Before launching or re-waking the instance, a start SHALL check that the
environment's weights are present in shared storage, judged by the same
completeness record the seeding writes rather than by the absence of an error.
While the weights are absent, a start SHALL NOT launch or re-wake the
instance, and SHALL report a retryable state that names the seed producing
the weights, so a caller can wait for the seed or follow it separately rather
than receiving an instance that boots against an incomplete prefix. When no
seed is running for those weights, the start SHALL start one, so that the
weights are produced rather than the start failing; a seed whose compute has
ceased — failed, stopped or reaped — does not count as running, and its
re-run follows the same identity and convergence rules as any other seed
request. A start that would exceed the cap on seeds in flight SHALL NOT start
another seed, and SHALL report the retryable state until a later start can.

The control plane SHALL request the engine's start on every path — a fresh
launch and a re-wake alike — once the instance's daemon answers its control
API, which on a fresh boot is the signal that the boot has stored the deploy
config; the boot's own user data SHALL NOT start the engine. The start SHALL
carry the deploy config as its body, so it always names the exact config the
daemon runs.

#### Scenario: A zone without capacity is not the end of it

- **WHEN** the first availability zone cannot provide the instance type
- **THEN** the remaining zones are tried before reporting failure

#### Scenario: Ready means serving

- **WHEN** a start request returns success
- **THEN** the model is answering requests at the environment's reported address

#### Scenario: No capacity anywhere

- **WHEN** every configured zone is out of capacity
- **THEN** the response says so, names the instance type it was trying, and
  indicates the caller may retry shortly

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
  provisioned throughput at the volume's ceiling and provisioned IOPS at four
  times that throughput

#### Scenario: A launch uses the environment's stored instance type

- **WHEN** an environment's deploy config names an instance type and a start
  launches a fresh instance for it
- **THEN** the instance is launched as that type

#### Scenario: A launch with no stored type uses the control plane default

- **WHEN** an environment's deploy config names no instance type and a start
  launches a fresh instance for it
- **THEN** the instance is launched as the control plane's default type

#### Scenario: A re-wake keeps the instance's original type

- **WHEN** an environment's instance is stopped, its deploy config's instance
  type is changed, and a start re-wakes the stopped instance
- **THEN** the instance comes back as the type it was originally launched
  with, because a stopped instance is not resized

#### Scenario: A changed type applies after the instance is terminated

- **WHEN** an environment's deploy config instance type is changed, its
  instance is terminated, and a later start launches it
- **THEN** the fresh instance is launched as the new type

#### Scenario: The control plane starts the engine on a fresh boot

- **WHEN** a fresh instance's daemon first answers its control API
- **THEN** the start request itself issues the engine's start, with the
  deploy config as its body, and reports ready only once the model answers —
  the boot started no engine

#### Scenario: A start while the weights are seeding launches nothing

- **WHEN** a start is requested for an environment whose weights are still
  being seeded
- **THEN** no instance of the environment is launched or re-woken, and the
  response is retryable and names the running seed

#### Scenario: A start with absent weights and no running seed starts one

- **WHEN** a start is requested for an environment whose weights are absent
  and no seed is running for them
- **THEN** a seed for those weights is started, no instance is launched, and
  the response is the same retryable state naming the seed

#### Scenario: Two starts for the same weights share one seed

- **WHEN** starts for two environments naming the same weights arrive while no
  seed is running
- **THEN** one seed is started, and both responses name it

#### Scenario: A ceased seed does not block a start

- **WHEN** the only seed for the weights has ceased — failed, stopped or
  reaped — and a start is requested
- **THEN** the weights are treated as absent: a new seed is started and the
  response is the retryable state, never a launch against the partial prefix
  the failed seed left behind

#### Scenario: A start that would exceed the seed cap waits

- **WHEN** the cap on seeds in flight is reached and a start needs to start a
  seed for absent weights
- **THEN** no further seed is started, and the response is retryable until a
  later start can start the seed

#### Scenario: A start proceeds once the weights are present

- **WHEN** a start has been reported in the seeding state and the seed has
  since finished, leaving the weights present
- **THEN** the next start launches the environment's instance and proceeds to
  ready as usual
