## MODIFIED Requirements

### Requirement: Bootstrap deploys the control plane

The system SHALL provide `spinloop remote bootstrap`, which deploys the
account-level control plane that every remote environment reuses — the EC2
Image Builder pipelines, the environment-aware lifecycle Lambdas and their IAM,
and the shared S3 weights bucket, IAM roles and VPC, and the IAM user that
holds the long-lived control-plane credential (see the Remote Auth
specification) together with its policy — by obtaining the CDK project shipped
in `remote/` and driving its deploy of the control-plane stack. Bootstrap SHALL
NOT start any AMI bake; the bake is a separate
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

#### Scenario: The control-plane credential user is deployed

- **WHEN** `spinloop remote bootstrap` completes
- **THEN** the control-plane IAM user exists with a policy covering the
  day-to-day remote commands only — invoking the control URLs, reading the
  control-plane log groups, describing the control-plane stack, and managing
  its own access keys — and no permission to deploy, bake, or otherwise
  provision AWS resources

#### Scenario: Bootstrap signposts the bake

- **WHEN** `spinloop remote bootstrap` completes
- **THEN** its output names `spinloop remote bake` as the next step, ahead of
  `spinloop remote deploy`

#### Scenario: Orchestration stops on a failed step

- **WHEN** any step in the sequence fails
- **THEN** bootstrap stops and reports which step failed rather than continuing
