## Context

See proposal.md for the motivation. The state this design works with:

- A seed's completeness is the `_seed.json` manifest the seeder writes as its
  final step; `weightsPresent(bucket, cfg)` in `remote/lambda/shared/seed.ts`
  answers "are these weights done" from it, including the companion check.
  This is the only judge of completeness the control plane has.
- A seed instance is discoverable by tag: `cloud-vm-llm: seed` plus
  `cloud-vm-llm:seed-id: <seedId>`, where `<seedId>` is derived from
  (runner, modelId, quant) by `seedIdFor` — the same inputs the start Lambda
  already holds in the environment's deploy-config.
- `launchSeedInstance` in `remote/lambda/shared/seed/launch.ts` is the one
  launch path: a deterministic idempotency token converges concurrent requests
  onto one instance, and a dead idempotency hit is escaped with a fresh
  generation. The deploy Lambda already calls it exactly this way, with the
  seed infrastructure read from its own environment (`seedInfraFromEnv`).
- `findManagedInstances` returns pending, running *and stopped* instances
  (stopped has to stay in for the endpoint re-wake path), so "a seed instance
  exists" is not the same as "a seed is in flight".
- The Go start client (`internal/remote.Start`) already loops on any 503 it
  gets, sleeping the reply's `retry_after_seconds`, until the context
  deadline. Both `spinloop remote start` and the fleet wake draw their
  progress line from `fleet.StartPhases` / `fleet.RenderPhase`.

## Goals / Non-Goals

**Goals:**

- A start never launches or re-wakes an instance whose weights are absent.
- While the weights are absent, a start either re-attaches to the running
  seed or starts one itself, and reports a retryable state naming the seed.
- One wording for the seeding state across `remote start` and the fleet
  dashboard, built the same way as every other state.
- No new commands, flags, or control-plane endpoints.

**Non-Goals:**

- `spinloop remote status` is unchanged: seed visibility stays on
  `seed ls` / `seed status`.
- No re-seeding of weights that *are* present — that remains the deliberate
  `--reseed` / `seed start --force` path.
- The streaming-Lambda vs EC2 thread from #97 is a separate decision.
- Deploy's auto-seed currently skips the concurrency cap; making it enforce
  the cap is a follow-up, not part of this change.

## Decisions

### 1. The gate checks the manifest, inside the wake path, before any instance work

`wake()` in `remote/lambda/start/index.ts` gains one check after it has read
the deploy-config and found the environment's EIP and security group, and
before it looks for an existing instance: if `weightsPresent(WEIGHTS_BUCKET,
deployConfig)` is false, the wake does not proceed to launch or re-wake.

The manifest is the right judge because it is the only record that says the
weights are *complete*: a seed's CloudWatch records stop arriving when its
process dies, and the absence of an error says nothing. Checking the seed's
state alone would answer "is a seed working on this" but not "is the work
done"; checking the prefix's contents would re-derive what the manifest
records. Placement after the deploy-config read is forced (the check needs
runner, model, quant and the companion map), and after the EIP/security-group
check keeps the existing "run `spinloop remote deploy`" answers for
never-deployed environments in front of anything about weights.

### 2. The start launches the seed itself, through the shared launch path

When the weights are absent and no seed is in flight, the start calls
`launchSeedInstance(buildSeedJob(deployConfig, infra, ''), infra)` — the same
two calls deploy makes, with the environment's full deploy-config so
companions ride along. The alternatives:

- **Refuse with "run `spinloop remote deploy`"** keeps start mutation-free,
  but makes start fail in a state only deploy can fix, for an environment
  that is already deployed — and fleet wakes have no operator standing by to
  run anything. It also leaves the adjacent case broken: a failed seed leaves
  a partial prefix in the bucket, and a plain launch against it is exactly
  the footgun this change closes.
- **Invoke the seed Lambda** would reuse its validation and cap, at the cost
  of a second wire format (parsing the seed Lambda's JSON reply inside the
  start Lambda) and an IAM edge between two Lambdas, for logic that is
  already a shared, tested module the deploy Lambda imports directly.

Auto-launching also answers #97's open question "auto-retry a stalled/failed
seed on next deploy/start, or require explicit `--force-reseed`": the answer
is auto-retry on the next start (deploy already has these semantics, so the
two paths agree), with the failure still diagnosable via
`spinloop remote seed status <id>`. A failed seed's dead instance is handled
by the launch path's existing escape: the constant `auto` idempotency token
returns the terminated instance, the launch sees a dead state, and retries
with a fresh generation — one new seed, converged.

A launch *failure* (for example, no capacity for the seed instance type)
returns the same retryable seeding state with the error in the message,
rather than a fatal 502 as deploy gives: a transient EC2 refusal of the seed
should not kill the start that wanted the model served, and the next poll
retries the launch. The convergence token means a retry can never double the
compute.

### 3. "In flight" means a tagged instance that is pending or running

The start filters seed instances by seed id and counts only `pending` and
`running` states as in flight — the same liveness rule the seed status join
uses. `findManagedInstances` includes `stopped`, and a stopped seed instance
is a dead seed (its joined state is already failed); treating it as in flight
would wedge every start for those weights behind a body that runs nothing.
The discovery filter moves from `remote/lambda/seed/index.ts` into
`remote/lambda/shared/seed/` so the seed and start Lambdas share one
definition of "find the seed instances".

Consulting CloudWatch records is deliberately not part of this check:
instance existence and state decide whether to block and whether to launch;
the join of records with existence is what `seed status` reports, and
re-deriving it per wake would add a log-group read to a hot path for a
verdict the instance state already gives.

### 4. The reply is a 503 `seeding` state, in the existing shape

```
503 { state: 'seeding', seedId: '<id>', retry_after_seconds: 60,
      message: 'seeding the weights — follow it with `spinloop remote seed status <id>`' }
```

The Go client loops on any 503 it receives, so no client control-flow change
is needed — only wording. `retry_after_seconds` of 60 matches the start
Lambda's existing waits; a seed runs for minutes, and polling faster buys
nothing. The `seedId` rides the reply (the `Response` struct already carries
it for deploy) so the CLI can name it without a second call.

The cap is enforced before launching: a start that finds at least
`MAX_CONCURRENT_SEEDS` seeds in flight returns the seeding state with a
message saying the cap was reached, and the next poll retries. That is a
queue, not a bypass — the cap still bounds compute, and the reply says what
is going on.

### 5. One new phase, drawn by both surfaces

`internal/fleet/start_phase.go` gains a `PhaseSeeding` kind, entered when
`onState` reports `seeding`, and rendered as `seeding the weights (Xm Ys)`
counting up from the first seeding reply — the same shape as `booting`,
since the existing progress-line parser only attaches a due time to a
capacity wait and a counting-up phase already answers "how long has this
been going on". `internal/remote.Start`'s 503 progress line special-cases
the state: the generic line reads `instance <state>`, and there is no
instance while the weights seed, so the seeding line names the seed instead.
If the context deadline expires, the give-up error names the state it last
saw; when that is seeding, it carries the seed's follow command and the
hint that a longer `--timeout` resumes the wait. The default 15-minute
timeout is unchanged: a seed takes 15–20 minutes, so a start that begins
one can outlast the default, and the honest answer is a clear message and a
safe re-run rather than silently waiting longer for everyone.

### 6. CDK: the start function gets the seed environment and three permissions

`remote/lib/llm-stack.ts` already assembles a `seedEnv` object — "everything
a Lambda needs to launch and supervise seeds" — currently spread into the
deploy and seed functions. The start function's environment gains
`...seedEnv` plus `MAX_CONCURRENT_SEEDS`, and its role gains the three
statements it lacks for the seed launch: `ssm:GetParameter` on the stock
AL2023 AMI parameter, `iam:PassRole` on the seed instance profile's role,
and read on the weights bucket's `models/*` prefix (for the manifest check;
the start Lambda has no weights-bucket grant today — the boot's S3 sync runs
under the instance profile, not the Lambda). `ec2:RunInstances`,
`ec2:CreateTags` and `ec2:DescribeInstances` are already granted. Code and
IAM ship together in one `remote bootstrap`, so there is no state where the
gate exists without its permissions.

## Risks / Trade-offs

- **A seed outlives the default start timeout** (15–20 min seed + several
  minutes boot > 15 min default) → the give-up message names the seed and
  the follow command; re-running start is safe (it re-attaches, converging
  on the same seed, and never launches a second instance); the operator can
  also just run `spinloop remote seed status <id>` and start later.
- **A persistently failing seed (bad model id, no HF access) is re-run on
  every start** → each attempt is bounded by the maximum seed lifetime and
  the failure is recorded where `seed status` reads it; the start's reply
  and the CLI line both name the seed while it runs. Deploy already has
  identical re-run semantics, so this changes no behaviour the account has
  not already accepted.
- **The start Lambda gains EC2 mutation scope it did not have** (it can now
  run seed instances) → the grants are the same ones the deploy and seed
  Lambdas hold: the instance profile is pass-able only to EC2, and the
  launch only ever happens for the seed id derived from the environment's
  own deploy-config, with the same convergence token deploy uses.
- **Eventual consistency between the manifest write and the next wake** → a
  wake that reads the manifest a heartbeat before the seeder's final write
  sees absent, re-attaches to the still-alive seed, and the next poll reads
  present. The seed writes the manifest only after every file is complete,
  so a wake can never observe "present but partial".
- **A stopped seed instance lingers past the sweep** → not counted as in
  flight, so a start launches a fresh seed rather than waiting on it; the
  sweep's existing seed pass still reaps the leftover.

## Migration Plan

Ship the CDK change with the Lambda code, then `spinloop remote bootstrap`
redeploys the control plane (code and IAM in one apply). No data migration:
the gate reads state that already exists (the manifest, the seed tags).
Rollback is a bootstrap with the previous version — the gate simply stops
being there and behaviour returns to today's.
