## Context

The daemon's start is the one seam every engine start goes through — a fresh
cloud boot, a re-wake, a fleet wake, `outfit serve --api` — so it is also the
seam where a page-cache pre-warm must be gated (spec: `serve-daemon`). Today
the daemon unconditionally pre-warms whenever a start names a model; the
cloud's boot user data and the start Lambda can both issue starts, and the
operator has no way to say "not this time". The control plane renders the
daemon's unit and the boot script, so both halves of the gate live in things
the control plane can reach.

## Goals / Non-Goals

**Goals:**
- Prewarm runs on cloud daemons by default, never on local or fleet daemons
  unless that machine's operator opts in by hand.
- The operator can disable the pre-warm for the specific wake they are asking
  for, from the CLI.
- One owner of the engine start (the control plane), so the operator's choice
  reaches every start, fresh boot included.

**Non-Goals:**
- Per-model or per-request pre-warm control (the grain is one start).
- Making fleet routing pre-warm on the operator's behalf (a fleet node's
  operator may run their daemon with the option; the fleet client never
  sets it).
- Changing how the weights are synced (the pre-warm reads what the sync
  wrote; it starts after the sync, never beside it).

## Decisions

**The option is a ceiling; the start may lower it, never raise it.**
`outfit daemon` gains `--prewarm` (default off), which installs the pre-warm
in the daemon; the deploy config a start carries gains a tri-state `prewarm`
field (`*bool` in Go, `boolean | undefined` in the shared TS type). Effective
pre-warm = daemon option AND (config field absent → true, else the field).
The cloud AMI's unit passes `--prewarm`, so the cloud default is on; a
`--no-prewarm` on `outfit remote start` renders `prewarm: false` into the
start's config; a local or fleet daemon never carries the flag, so no config
can light it up. Alternatives: a per-start field defaulting on everywhere
(would make a laptop or a fleet node read 26 GB on every start — the
original complaint); a start-call query parameter instead of a config field
(the deploy config is already the unit that says *what and how* a start runs,
and the start body is already push-then-start, so a new parameter beside it
would be a second way to say the same thing).

**The Lambda owns the start on every path, after boot signals complete.**
The boot user data keeps everything except the engine start — it syncs the
weights, writes the deploy config, enables the daemon (now with `--prewarm`),
and touches a boot-complete marker under the daemon's pinned config dir, in
place of today's thirty-try start loop. The start Lambda, after the SSM agent
is online, waits for boot readiness — the marker present, *or* the daemon
already reporting a running engine — and then issues the start itself, on a
fresh launch and a re-wake alike. The "or already running" half matters: an
instance booted by an older user data (which still starts its own engine)
must not wedge the new Lambda's wait. Alternatives: keep the boot's start and
have the Lambda only add its start on re-wake (then the operator's choice
cannot reach a fresh boot's start — the whole point of moving ownership);
have the Lambda poll the sync's progress directly (the marker is one SSM
check, and the boot is the only thing that knows the sync is done).

**The Lambda's start carries the rendered deploy config as its body, on
every path.** The Lambda already renders exactly this config for the user
data; the start body is the same render with the pre-warm resolved (the
operator's explicit choice, else the cloud default of true). A body-carrying
start persists the config before starting, which is the existing
push-then-start contract — the values are the same ones boot wrote, and the
engine key travels with it, keeping the "the key travels with the config it
arrived with" invariant. Alternative: a bodyless start plus a separate
pre-warm parameter (two ways to say what the start runs; and on a re-wake the
stored config and the start's choice would diverge with nothing to reconcile
them).

**Provisioned throughput only, sized from the AMI's own mapping.** A
RunInstances block device mapping replaces the AMI's root mapping, so the
launch must repeat the AMI's own root size (read from the same
`DescribeImages` call `findLatestAmi` already makes) or it would launch an
undersized default. Throughput goes to the gp3 ceiling (1,000 MiB/s); IOPS
stays at the 3,000 baseline, because with the pre-warm the read is
sequential and 3,000 × 4 KB is two orders of magnitude below the
throughput ceiling. The seed instance is left unprovisioned: its transfer is
network-bound, not EBS-bound.

## Risks / Trade-offs

- [The Lambda dies between launching the instance and issuing the start] →
  the instance sits with its engine stopped until the next start attempt; the
  CLI already retries on 503, and a re-wake has always had this property —
  moving the fresh boot onto it makes the two paths behave the same, which is
  the point. The baked crash-nudge intentionally does not start a *stopped*
  engine (only a crashed one), so readiness stays the control plane's to own.
- [A fresh boot's sync fails] → the marker never appears; the Lambda hits its
  deadline and returns the same retryable 503 it does today when a crashing
  engine never answers health. Remediation is unchanged (redeploy, or
  terminate and let the next start relaunch).
- [A new Lambda launches an instance from an older baked AMI] → the unit the
  Lambda renders carries `--prewarm`, which the older outfit binary refuses,
  and that daemon never starts. Mitigation is ordering: bake the runtime AMI
  with the new outfit release before deploying the control plane — the
  established convention for exactly this shape of change (the control plane
  deploys after the images that carry what it needs). The boot-readiness wait
  tolerating an already-running engine covers in-flight instances of older
  user data; it cannot cover the flag, and must not pretend to.
- [Rolling the control plane back after a rebake] → a new-AMI instance's boot
  no longer starts its own engine, so the old Lambda would leave it idle; the
  rollback is rebake-then-redeploy, in that order. The window is short and the
  environment reports 503 until it is done, rather than failing silently.
- [The pre-warm competes with the sync for the volume] → it cannot: the
  start (and so the pre-warm) is issued only after the marker, which is
  written after the sync.

## Migration Plan

1. Merge; cut the outfit release carrying the daemon option, the config
   field, and the CLI flags.
2. `pnpm bake` the runtime AMIs (the bake picks up the new release's pinned
   version).
3. `pnpm deploy` the control plane (the Lambda changes).
4. The next wake of each environment exercises the new path; nothing is
   forced — an environment keeps its behaviour on its next start.

## Open Questions

None — the option's grain (one start), its ceiling semantics, the cloud
default, and the single owner of the start are all fixed by the requirements.
