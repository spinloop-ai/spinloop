## Context

A cold wake pays its root volume twice: the S3 sync writes the weights
through it, and the engine's load faults them back in page by page — both
against a gp3 baseline of 3,000 IOPS and 125 MiB/s. And the engine's first
start had two owners: the boot's own user-data start loop on a fresh boot,
and the start Lambda on a re-wake — so the two paths differed, and nothing
about a fresh boot's start was reachable from the outside.

## Goals / Non-Goals

**Goals:**
- Provision the root volume at the launch, at the gp3 ceiling, so the
  weights sync and the model load run at the volume's real limits.
- One owner of the engine start (the control plane), on every path — a
  fresh launch and a re-wake alike.

**Non-Goals:**
- Changing how the weights are synced.
- Making the model load faster than its volume: the load faults pages in
  one at a time, which is an IOPS story, not a bandwidth story (the
  page-cache pre-warm that tried to make it a bandwidth story was evaluated
  and removed — see the decision below).

## Decisions

**The Lambda owns the start on every path, once the daemon answers.**
The boot user data keeps everything except the engine start — it syncs the
weights, writes the deploy config, and enables the daemon — and stops; the
thirty-try start loop is gone. The boot writes the deploy
config *before* it enables the daemon, so the daemon's first answer to its
control API is the boot's signal that the config is stored: no separate
marker file to write, lose or interpret. The start Lambda, after the SSM
agent is online, waits for that first answer (any state — the daemon answers
before the engine exists, and stays up when it is stopped) and then issues
the start itself, on a fresh launch and a re-wake alike. The wait also covers
the in-flight case: an instance booted by an older user data (which still
runs its own start loop) answers the same way, and the Lambda's start meets
the engine the old boot started with a 409, which is the idempotent start
doing its job. Alternatives: keep the boot's start and have the Lambda only
add its start on re-wake (then a fresh boot's start is unreachable from the
outside — the whole point of moving ownership); a boot-complete marker
file (one more thing to write, and the daemon's answer already implies it —
the daemon starts only after the config is written, which is what the marker
would have said).

**The Lambda's start carries the rendered deploy config as its body, on
every path.** The Lambda already renders exactly this config for the user
data; the start body is the same render. A body-carrying
start persists the config before starting, which is the existing
push-then-start contract — the values are the same ones boot wrote, and the
engine key travels with it, keeping the "the key travels with the config it
arrived with" invariant. Alternative: a bodyless start plus separate start
parameters (two ways to say what the start runs; and on a re-wake the
stored config and the start's parameters would diverge with nothing to
reconcile them).

**Provisioned throughput and IOPS at the ceiling, sized from the AMI's own
mapping.** A RunInstances block device mapping replaces the AMI's root
mapping, so the launch must repeat the AMI's own root size (read from the same
`DescribeImages` call `findLatestAmi` already makes) or it would launch an
undersized default. Throughput goes to the gp3 ceiling (1,000 MiB/s); EC2
caps gp3 throughput at 0.25 MiB/s per provisioned IOP, so the ceiling needs
4,000 IOPS — 1,000 above the baseline, the price of the last 250 MiB/s,
billed only while the volume exists (a few dollars a month against a
$1.86/hour GPU). The IOPS is not merely the price of the throughput: the
engine's load is page-fault-bound, and it is the provisioned IOPS — not the
throughput — that move it. The seed instance is left unprovisioned: its
transfer is network-bound, not EBS-bound.

**The page-cache pre-warm was built, live-checked, and removed.** The
original design had the daemon read the model's files sequentially, in the
background, ahead of the engine's load — on the theory that the load's copy
to the GPU was bandwidth-bound and a sequential read could get ahead of the
faults. On a live instance (g6e.xlarge: 32 GB RAM, a ~30 GB model, the
provisioned root) the read could not get ahead: the engine maps its weights
and faults them in from the first second of the load, and a provisioned gp3
root hands out at most 4,000 IOPS whatever else wants the disk, so the
faults owned the volume from the start and the pre-warm's own `read_bytes`
went flat after a ~1.5 GB burst in the first seconds. And a pre-warm that
finished in time would not have helped: 32 GB of RAM cannot hold a ~30 GB
model in the page cache, so the faults would only have moved to a different
second. Measured 2026-08-23: the model loaded in ~115 s with the pre-warm
on, and off it was no faster — the pre-warm merely double-read ~1.5 GB. The
feature was removed with its whole plumbing (the daemon's flag, the
deploy-config field, the CLI's flags, the control plane's parameter), and
the provisioned volume kept: the S3 sync is the one reader whose limit is
the volume's throughput, and the IOPS ceiling is what the load takes.

## Risks / Trade-offs

- [The Lambda dies between launching the instance and issuing the start] →
  the instance sits with its engine stopped until the next start attempt; the
  CLI already retries on 503, and a re-wake has always had this property —
  moving the fresh boot onto it makes the two paths behave the same, which is
  the point. The baked crash-nudge intentionally does not start a *stopped*
  engine (only a crashed one), so readiness stays the control plane's to own.
- [A fresh boot's sync fails] → the daemon is never enabled, so it never
  answers; the Lambda hits its deadline and returns the same retryable 503 it
  does today when a crashing engine never answers health. Remediation is
  unchanged (redeploy, or terminate and let the next start relaunch).
- [A new Lambda launches an instance from an older baked AMI] → the user
  data the Lambda renders is plain: the daemon unit uses only long-standing
  flags and the stored deploy config keeps its shape, so an older spinloop
  binary boots under it. (The original form of this risk — a rendered unit
  carrying a flag the older binary would refuse — went away with the
  pre-warm, which is also why this change needs no rebake.)
- [Rolling the control plane back] → an instance booted under the new user
  data no longer starts its own engine, so the old Lambda would leave it
  idle; the rollback is to redeploy the old control plane, and until it is
  done the environment reports 503 rather than failing silently.

## Migration Plan

1. Merge.
2. `pnpm deploy` the control plane.
3. The next wake of each environment exercises the new path; nothing is
   forced — an environment keeps its current behaviour until its next start.

## Open Questions

None — the volume provisioning and the single owner of the start are fixed
by the requirements, and the pre-warm question, which was left open by
design, is answered by the live check and closed by the removal.
