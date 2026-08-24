## Why

A remote wake's model load is its dominant cost. The engine maps the weights
and faults them in page by page as it copies them to the GPU, against the
instance's gp3 root volume — and the same volume throttles the cold wake's
S3 sync of those weights. An unprovisioned gp3's baseline is 3,000 IOPS and
125 MiB/s, so a ~26 GB model takes around ten minutes to load, and a cold
wake runs to roughly fifteen. The volume can be asked for its real ceiling,
and the two paths that start the engine can be made one.

## What Changes

- **The launch provisions the root volume.** The start Lambda launches the
  instance with a gp3 block device mapping at 1,000 MiB/s of provisioned
  throughput, sized from the AMI's own root mapping (a RunInstances mapping
  replaces the AMI's, so the size is read from the AMI). IOPS go to the 4,000
  EC2 requires for that throughput — gp3 caps throughput at 0.25 MiB/s per
  provisioned IOP, so the ceiling needs 4× it in IOPS.

- **The control plane owns the engine start.** The instance's boot no longer
  requests the engine's first start: it writes the deploy config, enables
  the daemon, and signals boot complete. The start Lambda issues the daemon's
  start request on every path — fresh launch and re-wake alike — exactly as
  it already does for a re-wake, and it carries the deploy config as its
  body.

- **The page-cache pre-warm was built, live-checked, and removed.** The
  first design read the model's files ahead of the engine's load, on the
  theory that the load was bandwidth-bound and a sequential read could get
  ahead of the faults. On a live instance the read cannot get ahead: the
  engine faults pages in from the first second of the load, a provisioned
  gp3 root hands out at most 4,000 IOPS whatever else wants the disk, and
  the box's 32 GB of RAM cannot hold a ~30 GB model in the page cache at
  all. The pre-warm cost a ~1.5 GB double-read and saved no time, so the
  feature and its whole plumbing (the daemon flag, the deploy-config field,
  the CLI flags, the control plane's parameter) came out before this change
  ships. The provisioned volume is what remains of the load-speed work — it
  is the S3 sync's limit, and its IOPS ceiling is the load's.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `remote-engine-host`: the boot sequence no longer requests the engine's
  first start — the control plane does, on every path.
- `endpoint-lifecycle`: a launch provisions the root volume's throughput
  and IOPS, and the start request reports ready only after the control
  plane itself has started the engine, once boot has signalled complete.

## Impact

- **remote/**: the start Lambda (the block device mapping, the wait for the
  daemon to answer, the start request's body); `runners/daemon-boot.ts`
  (the dropped start loop); `shared/daemon.ts` (the start command with its
  body); `docs/costs.md` (the provisioned throughput and IOPS).
- **Cost**: provisioned throughput and IOPS add roughly $9.40/month,
  pro-rated to time up (875 MiB/s over the baseline at $0.005/MiB-month,
  and 1,000 IOPS at $0.005/IOP-month).
- **Release coupling**: none beyond the control plane itself. The unit the
  Lambda renders uses only long-standing daemon flags and the deploy config
  keeps its shape, so no runtime AMI rebake is required and no older binary
  can refuse what this change ships; each environment picks up the new
  behaviour on its next wake.
