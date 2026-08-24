## REMOVED Requirements

### Requirement: Outfit is baked into the runtime AMI

**Reason**: Baking outfit into the AMI ties every daemon change to a slow
image-build pipeline, forcing a re-bake for changes that touch nothing else
about the instance. Outfit is now fetched and installed during boot instead
(see "Outfit is installed during boot, not baked into the AMI").

**Migration**: No action needed for existing environments — the next fresh
launch of an environment's instance installs outfit during boot instead of
finding it already on the AMI. Environments do not need to be redeployed for
this change; only a fresh instance launch (not a mere re-wake of a stopped
one) picks up the new boot behavior, since a re-wake reuses whatever outfit
version its disk already has installed.

## ADDED Requirements

### Requirement: Outfit is installed during boot, not baked into the AMI

The runtime AMIs for both runners SHALL NOT include a pinned outfit release.
Instead, the instance's boot sequence SHALL fetch and install the outfit
binary early — before the `outfit-daemon` systemd unit is written and
enabled — so the daemon has a binary to run once its unit starts. An AMI
bake SHALL NOT need to change when only outfit's own code changes.

#### Scenario: The AMI carries no outfit binary or version pin

- **WHEN** a runtime AMI is built
- **THEN** it contains no outfit binary and declares no pinned outfit version
  among its bake parameters

#### Scenario: Boot installs outfit before the daemon can start

- **WHEN** an instance launches from a runtime AMI
- **THEN** the boot sequence installs the outfit binary before the
  `outfit-daemon` systemd unit is written and enabled

#### Scenario: A daemon-only change needs no new AMI

- **WHEN** an outfit release ships a daemon-only change
- **THEN** an environment's instances pick it up through a fresh boot,
  without any runtime AMI being rebuilt

### Requirement: The outfit install is idempotent

The boot's outfit install step SHALL be safe to run more than once: given the
same resolved version, re-running it SHALL succeed without re-downloading an
already-correct install, and an interrupted attempt SHALL never leave a
partially-installed or corrupted binary in place. The downloaded release
SHALL be checksum-verified against its own published checksums before
installation is considered complete, matching the verification the AMI bake
performed previously.

#### Scenario: Re-running the install step is a no-op when already correct

- **WHEN** the boot's install step runs again for a version already correctly
  installed
- **THEN** it leaves the existing binary in place unchanged and does not fail

#### Scenario: An interrupted install never leaves a broken binary

- **WHEN** the install step is interrupted while downloading or verifying the
  release
- **THEN** the previous state (no binary, or a previously verified one) is
  preserved — never a partially-written or unverified binary

#### Scenario: The downloaded binary is checksum-verified

- **WHEN** boot downloads an outfit release
- **THEN** it verifies the download against the release's published checksums
  before installing it, and fails the boot loudly on a mismatch rather than
  installing an unverified binary

### Requirement: Outfit version resolves from the deploy config, defaulting to latest

The version of outfit a boot installs SHALL come from the environment's
deploy config: an explicit pinned version SHALL install exactly that release,
and an absent pin SHALL install the latest published release. A pinned
version that does not exist as a release SHALL fail the boot loudly rather
than silently falling back to another version.

#### Scenario: No pin installs latest

- **WHEN** an environment's deploy config names no outfit version
- **THEN** the boot installs the latest published outfit release

#### Scenario: A pin installs exactly that version

- **WHEN** an environment's deploy config pins an outfit version
- **THEN** the boot installs exactly that release, not the latest

#### Scenario: An unresolvable pin fails loudly

- **WHEN** a pinned version names a release that does not exist
- **THEN** the boot fails with a clear error rather than installing a
  different version in its place

#### Scenario: A version pin change takes effect on the next fresh launch

- **WHEN** an operator changes an environment's pinned outfit version while
  its instance is stopped
- **THEN** the change is not applied until the environment's instance is next
  freshly launched — a re-wake of the existing stopped instance keeps
  whatever outfit version it last installed at boot
