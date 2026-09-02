## ADDED Requirements

### Requirement: AMI bake is a separate command

The system SHALL provide `spinloop remote bake`, which starts an AMI bake for
each runner named as a positional argument — `llamacpp` and `vllm` — defaulting
to both when none are named. It SHALL drive the same CDK project that
bootstrap orchestrates, with the same version-matched source download into the
same ref-keyed default location, the same package-manager selection and
override, and `--ref` and `--dir` flags matching bootstrap's. Bake SHALL NOT
deploy any stack; when the control-plane stack is not deployed, it SHALL fail
before starting any bake, naming `spinloop remote bootstrap` as the step to run
first. Bake SHALL block until every requested runner's AMI is available; a
`--no-wait` flag SHALL return as soon as the bakes are queued, reporting how to
check on them, rather than blocking for the bake duration.

#### Scenario: Default bake covers both runners

- **WHEN** the user runs `spinloop remote bake` with no arguments
- **THEN** a bake is started for both `llamacpp` and `vllm`

#### Scenario: A single runner is baked

- **WHEN** the user runs `spinloop remote bake llamacpp`
- **THEN** only the `llamacpp` AMI bake is started

#### Scenario: An unknown runner is rejected

- **WHEN** the user names a runner that is neither `llamacpp` nor `vllm`
- **THEN** bake fails before starting any bake, naming the accepted runners

#### Scenario: No control plane

- **WHEN** the control-plane stack is not deployed and bake runs
- **THEN** it fails before starting any bake, saying to run
  `spinloop remote bootstrap` first

#### Scenario: Bake waits by default

- **WHEN** the user runs `spinloop remote bake` without `--no-wait`
- **THEN** the command blocks until the requested runners' AMIs are available
  before finishing

#### Scenario: Handing off with --no-wait

- **WHEN** the user passes `--no-wait`
- **THEN** the command returns as soon as the bakes are queued, reporting how
  to check on them, rather than blocking for the bake duration

### Requirement: Idempotent bootstrap

Bootstrap SHALL be safe to re-run: it SHALL skip the package-manager install when
dependencies are present and `cdk bootstrap` when the account and region are already
bootstrapped, and SHALL not redeploy a control-plane stack that is unchanged. Because it
touches only control plane and never a live instance, re-running SHALL NOT
require any override.

#### Scenario: Re-running skips satisfied steps

- **WHEN** bootstrap is re-run
- **THEN** it skips installation and CDK bootstrap that are already done and
  no-ops the unchanged control-plane stack, without requiring an override

### Requirement: Bootstrap collects only the shared-secret token

Bootstrap SHALL collect the one control-plane setting the CDK has no default
for and write it where the CDK reads it: an optional Hugging Face token for
the shared secret used when seeding gated weights. Which runner AMIs to bake
is not a bootstrap setting — the engine is a per-environment choice made at
`deploy`, and the runners are named by `spinloop remote bake` itself. The
allowed ingress CIDR is also per-environment and belongs to `deploy`, not here.

#### Scenario: Runners are not a bootstrap setting

- **WHEN** the user runs bootstrap
- **THEN** no runner selection is requested or written, since the runners are
  named at `spinloop remote bake`

#### Scenario: The allowed CIDR is not a bootstrap setting

- **WHEN** the user runs bootstrap
- **THEN** no ingress CIDR is requested or written, since it is scoped per
  environment at `spinloop remote deploy`

## MODIFIED Requirements

### Requirement: Bootstrap deploys the control plane

The system SHALL provide `spinloop remote bootstrap`, which deploys the
account-level control plane that every remote environment reuses — the EC2
Image Builder pipelines, the environment-aware lifecycle Lambdas and their IAM,
and the shared S3 weights bucket, IAM roles and VPC — by obtaining the CDK
project shipped in `remote/` and driving its deploy of the control-plane stack.
Bootstrap SHALL NOT start any AMI bake; the bake is a separate
`spinloop remote bake` step. Bootstrap SHALL NOT create any Elastic IP or EC2
instance, and SHALL NOT register an environment; those belong to
`spinloop remote deploy`. Bootstrap SHALL NOT reimplement the infrastructure;
it SHALL orchestrate the existing CDK project. On success, bootstrap SHALL
signpost `spinloop remote bake` as the next step, ahead of
`spinloop remote deploy`.

#### Scenario: A successful bootstrap yields the control plane

- **WHEN** `spinloop remote bootstrap` completes
- **THEN** the control-plane stack is deployed — Image Builder pipelines, the
  lifecycle Lambdas, and the shared bucket/roles/VPC — with no Elastic IP or
  instance created and no AMI bake started

#### Scenario: Bootstrap signposts the bake

- **WHEN** `spinloop remote bootstrap` completes
- **THEN** its output names `spinloop remote bake` as the next step, ahead of
  `spinloop remote deploy`

#### Scenario: Orchestration stops on a failed step

- **WHEN** any step in the sequence fails
- **THEN** bootstrap stops and reports which step failed rather than continuing

### Requirement: Version-matched CDK sources

Bootstrap and `spinloop remote bake` SHALL obtain the CDK project by
downloading the `remote/` tree from the project repository at a reference
matching the running binary's version, so the infrastructure matches the CLI
driving it. A `--ref` flag SHALL override the reference, and a `--dir` flag
SHALL override where the sources are placed (defaulting under the user config
directory). For a development build with no release version, bootstrap and
bake SHALL fall back to a documented default reference. The CDK sources SHALL
NOT be embedded in the binary, since a package-manager install is required at
runtime regardless.

The default source location SHALL be keyed by the resolved reference, so a re-run
at the same version reuses its sources while a different binary version downloads
fresh. On a successful bootstrap or bake using the default location, sources
from other references SHALL be pruned. An explicit `--dir` SHALL be treated as
the user's own location: neither keyed by reference nor pruned.

#### Scenario: Sources match the binary version

- **WHEN** a released `spinloop` runs bootstrap with no `--ref`
- **THEN** it downloads the `remote/` sources at the tag matching its version

#### Scenario: Development build falls back

- **WHEN** a `dev` build runs bootstrap with no `--ref`
- **THEN** it uses the documented fallback reference rather than guessing

#### Scenario: A new version does not reuse stale sources

- **WHEN** bootstrap or bake runs from a binary whose resolved reference
  differs from a previously downloaded one in the default location
- **THEN** it downloads sources for the new reference, and on success the
  superseded reference's sources are pruned

### Requirement: A Node package manager is selected, overridable, and logged

Bootstrap and `spinloop remote bake` SHALL select the Node package manager they
drive the CDK project with. Absent an explicit choice, they SHALL auto-detect by
PATH lookup, preferring `pnpm` and falling back to `npm` when `pnpm` is not on
the path. The user MAY override
the selection with a `--package-manager` flag or an `SPINLOOP_REMOTE_PACKAGE_MANAGER`
environment variable, whose only accepted values are `pnpm` and `npm`; the flag
SHALL take precedence over the environment variable, which SHALL take precedence
over auto-detection. An unrecognised override value SHALL be rejected with an
error naming the accepted values. The selected manager SHALL be used consistently
for every Node step (install, `cdk`, `deploy:image`, `deploy`) and
reflected in the printed plan. Before the steps run, bootstrap SHALL log which
package manager it selected, so the run is self-explanatory. When auto-detecting
and both managers are present, `pnpm` SHALL win; the choice SHALL NOT depend on
which lockfiles are present, since the `remote/` project ships a `pnpm` lockfile
yet runs correctly under either manager.

#### Scenario: pnpm is preferred when auto-detecting

- **WHEN** no override is given and both `pnpm` and `npm` are on the path
- **THEN** bootstrap selects `pnpm`, logs that it is using `pnpm`, and runs every
  step with `pnpm`

#### Scenario: npm is used when pnpm is absent

- **WHEN** no override is given, `pnpm` is not on the path, but `npm` is
- **THEN** bootstrap selects `npm`, logs that it is using `npm`, and runs every
  step with `npm` using npm's argument conventions

#### Scenario: An explicit override is honoured

- **WHEN** the user passes `--package-manager npm` (or sets
  `SPINLOOP_REMOTE_PACKAGE_MANAGER=npm`) while `pnpm` is also present
- **THEN** bootstrap uses `npm` regardless of auto-detection, and the flag wins
  if both the flag and the environment variable are set

#### Scenario: An unrecognised override is rejected

- **WHEN** the override value is neither `pnpm` nor `npm`
- **THEN** bootstrap fails with an error naming the accepted values, before
  deploying anything

## REMOVED Requirements

### Requirement: Control-plane settings are collected

**Reason**: The per-runner setting it collected (`--runners`) existed to feed
the bake, which is now a separate command that names its own runners; what
remains is only the Hugging Face token.

**Migration**: Runner selection moves to `spinloop remote bake`'s positional
arguments (defaulting to both runners); what bootstrap still collects is
described by the re-added "Bootstrap collects only the shared-secret token"
requirement.

### Requirement: Idempotent bootstrap with an asynchronous bake

**Reason**: Bootstrap no longer starts the AMI bake, so there is no
asynchronous bake to hand off or wait for; its re-run idempotency remains,
described without the bake.

**Migration**: Re-run behaviour is carried by the "Idempotent bootstrap"
requirement; baking and waiting for it is `spinloop remote bake` (which waits
by default), per the "AMI bake is a separate command" requirement.
