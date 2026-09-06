## Why

`spinloop remote start` never checks whether the environment's weights are in
S3. Run while the model's seed is still transferring, it launches the runtime
instance, which syncs the *partial* prefix the seed has written so far and
boots against incomplete weights — the footgun the deploy reply's warning
describes, with no guard behind it (gap 3 of #97, the one the seed rework left
open). The same is true after a seed has failed: the partial prefix is still in
the bucket, and a start happily launches against it.

## What Changes

- The start path checks that the environment's weights are present (the
  manifest the seeder writes as its final step) before it launches or re-wakes
  the environment's instance.
- While the weights are absent, a start launches nothing. It reports a
  retryable `seeding` state naming the seed — the same 503/retry-after shape as
  the existing `starting` and `no-capacity` states — so the existing polling
  loop waits instead of the instance booting on an incomplete prefix.
- If no seed is already running for those weights, the start launches one
  itself, through the same shared launch path deploy uses: the deterministic
  idempotency token still converges concurrent requests onto one instance, and
  the same cap on seeds in flight applies. A failed or never-run seed is
  therefore re-run on the next start, and a start interrupted mid-seed simply
  re-attaches and keeps waiting.
- The CLI recognises the `seeding` state: `spinloop remote start` (and the
  fleet wake, which reuses the same start surface) shows a line that names the
  seed, and a timeout that expires mid-seed says how to follow the seed and
  how to resume waiting rather than a generic "gave up".
- The start Lambda gains the environment, permissions and read access to the
  weights bucket it needs to make the check and launch a seed.

No new commands or flags: `spinloop remote seed` is unchanged, and `start`
keeps its existing interface.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `endpoint-lifecycle`: "Starting on demand" gains a precondition — a start
  does not launch an instance whose weights are not yet in S3; it reports a
  retryable seeding state that names the seed, and it ensures a seed is in
  flight when one is not.

## Impact

- `remote/lambda/start/index.ts` — the weights gate in the wake path.
- `remote/lambda/seed/index.ts` + `remote/lambda/shared/seed/` — the seed
  instance discovery the seed Lambda has inlined moves to the shared seed
  modules so the start path uses the same one.
- `remote/lib/llm-stack.ts` — the start function's environment, IAM (launching
  and supervising seeds, reading the weights bucket) and, for tests, the
  matching CDK assertions.
- `internal/remote/remote.go` — the start client's handling of the `seeding`
  state and its timeout message.
- `internal/fleet/start_phase.go` — a phase for the seeding state, so the
  dashboard and `remote start` word it the same way as every other state.
- Existing deployments need a `spinloop remote bootstrap` after the new CDK
  ships for the start Lambda to gain its permissions; until then the gate is
  inert and behaviour is unchanged.
