## Why

Every change to outfit's daemon currently requires re-baking both runtime AMIs
before it can reach a remote instance — a slow, GPU-adjacent build pipeline
whose only job is to lay down a single small Go binary. Outfit is a few
megabytes and takes seconds to fetch; baking it into the AMI ties a fast,
frequent release cycle to a slow, infrequent image-build cycle, and the idle
check's own compatibility note (in `remote-engine-host`) already warns that
AMIs must be re-baked before the control plane deploys, for exactly this
reason. Pulling outfit at boot instead removes that coupling: daemon changes
ship the moment a release is cut, no bake required.

## What Changes

- The image bake **no longer installs outfit**: the "outfit itself" step is
  removed from the shared bake preamble (`image-stack.ts`), along with the
  `OutfitVersion` Image Builder parameter and the `outfitVersion` synth-time
  config check that exists solely to feed it. **BREAKING** for the image
  stack's bake parameters, not for any deployed environment.
- The instance's boot sequence installs outfit itself, early — before the
  daemon's systemd unit is enabled — using the same download-and-checksum-
  verify approach the bake used, now run as a `curl | verify | install` step
  in user data ahead of `daemonBoot()`.
- The install is **idempotent**: safe to run again without re-downloading or
  breaking an already-correct install (e.g. a retried boot step), matching
  the idempotency already promised elsewhere in this boot sequence (the swap
  file setup, the CloudWatch agent config write).
- By default the boot installs the `latest` GitHub release. A deployment may
  pin an exact version instead: `outfit remote deploy` gains an optional
  version-pin input that flows into the environment's stored deploy config,
  and the boot resolves that pin (or `latest` when unset) to a concrete
  download URL.
- `outfit remote deploy --dry-run` and the deploy plan output surface the
  resolved outfit version (pinned or `latest`) alongside the runner and model
  it already prints.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `remote-engine-host`: outfit is no longer baked into the AMI; instead the
  boot sequence fetches and installs it idempotently, before the daemon's
  unit starts, resolving `latest` or a pinned version from the deploy config.
- `environment-deployment`: `outfit remote deploy` accepts an optional
  outfit-version pin, stored in the environment's deploy config and shown in
  the deploy plan.

## Impact

- `remote/lib/image-stack.ts`: drop the outfit bake step, the `OutfitVersion`
  Image Builder parameter, and the `outfitVersion` config/synth check.
- `remote/lambda/runners/daemon-boot.ts` and `remote/lambda/start/index.ts`
  (`buildInferenceUserData`): add the boot-time install step ahead of
  `daemonBoot()`.
- `remote/lambda/shared/deploy-config.ts`: add an optional
  `outfitVersion` field to `DeployConfig`, validated and defaulted to
  `latest`.
- `cmd/outfit/remote.go` and the Go-side `remote.DeployConfig` type: add a
  deploy-time flag to set the pin, and print it in the deploy plan.
- `remote/lib/config.ts`: the CDK-level `outfitVersion` context setting
  (used only for the bake) becomes unused and is removed.
- Documentation: `remote/docs/architecture.md` ("Image stack" section, which
  currently says outfit is baked in) and `docs/commands/remote.md` (deploy
  flags table).
