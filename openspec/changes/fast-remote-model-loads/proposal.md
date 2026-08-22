## Why

A remote wake's model load is its dominant cost. The engine maps the weights
and faults in 4 KB pages one at a time as it copies them to the GPU —
latency-bound I/O against the instance's gp3 root volume, whose unprovisioned
baseline is 3,000 IOPS and 125 MiB/s. A ~26 GB model therefore takes around
ten minutes to load, and a cold wake runs to roughly fifteen. Both halves are
fixable: the volume can be asked for its real throughput ceiling, and the
read can be made sequential.

## What Changes

- **The launch provisions the root volume.** The start Lambda launches the
  instance with a gp3 block device mapping at 1,000 MiB/s of provisioned
  throughput, sized from the AMI's own root mapping (a RunInstances mapping
  replaces the AMI's, so the size is read from the AMI). IOPS stay at the
  baseline: with a sequential read, throughput is the whole limit.

- **The daemon pre-warms the page cache.** When the daemon starts an engine
  it first reads the model's files — the deploy config's model path, a file
  or a directory — sequentially in the background, so the engine's copy to
  the GPU is mostly cache hits instead of network faults. The read is
  best-effort: it never blocks the start, it fails silently, and a missing
  path is a no-op.

- **Prewarm is a daemon option, on only for cloud daemons.** `outfit daemon`
  gains a pre-warm option, off by default. The cloud runtime AMI's daemon
  unit passes it, so a cloud daemon pre-warms by default; a local
  `outfit daemon`, `outfit serve --api`, and a fleet node never do. A start
  request may disable the pre-warm for itself, but may not enable it where
  the daemon does not run with the option.

- **The pre-warm choice rides the start request.** The deploy config a start
  carries gains an optional pre-warm field. On the cloud the default is
  enabled, and `outfit remote start` (and `restart`, which composes a stop
  and a start) accept the choice, so the operator can skip the pre-warm for
  the wake they are asking for.

- **The control plane owns the engine start.** The instance's boot no longer
  requests the engine's first start: it writes the deploy config, enables
  the daemon (with pre-warm), and signals boot complete. The start Lambda
  issues the daemon's start request on every path — fresh launch and re-wake
  alike — exactly as it already does for a re-wake, and it carries the
  pre-warm choice.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `serve-daemon`: the daemon command gains the pre-warm option; when enabled,
  an engine start pre-warms the model's page cache; the option is a ceiling a
  start request may lower but never raise.
- `remote-engine-host`: the boot sequence no longer requests the engine's
  first start — the control plane does, on every path — and the daemon the
  boot enables runs with the pre-warm option.
- `endpoint-lifecycle`: a launch provisions the root volume's throughput, and
  the start request reports ready only after the control plane itself has
  started the engine, once boot has signalled complete.
- `daemon-api`: the deploy config a start carries may name the pre-warm
  choice for that start.
- `remote-endpoint`: `outfit remote start` and `restart` accept the pre-warm
  choice, defaulting to the cloud default.

## Impact

- **Go**: `internal/daemon` (the pre-warm, the option's wiring, honouring the
  per-start choice); `internal/remote` (`DeployConfig`'s pre-warm field and
  the start request's choice); `cmd/outfit` (the daemon option, the remote
  start/restart flags); the daemon's OpenAPI description and its contract
  test for the new field.
- **remote/**: the start Lambda (the block device mapping, the wait for boot
  to signal complete, the start request's body, the pre-warm parameter);
  `runners/daemon-boot.ts` (the daemon unit's option, the boot-complete
  marker in place of the start loop); `shared/daemon.ts` (the start command
  with its body); the shared deploy-config type.
- **Cost**: provisioned throughput adds roughly $4.40/month, pro-rated to
  time up (875 MiB/s over the baseline at $0.005/MiB-month).
- **Release coupling**: the pre-warm and the boot change live in the baked
  runtime AMI, so they take effect only after an outfit release and a
  rebake; the control plane ships separately and must follow the rebake,
  because the daemon unit it renders carries the new option, which an older
  outfit binary would refuse.
