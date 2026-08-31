## ADDED Requirements

### Requirement: Deploy accepts an optional spinloop version pin

`spinloop remote deploy` SHALL accept an optional flag pinning the exact spinloop
release the environment's instances install at boot. When the flag is given,
deploy SHALL record that version in the environment's stored deploy config so
the next fresh boot installs it; when it is absent, deploy SHALL record no pin
and the environment's boots install the latest published release (the boot's
default, defined by the `remote-engine-host` capability). An empty or
whitespace-only value SHALL be treated as if the flag were not given.

#### Scenario: A pin is recorded in the deploy config

- **WHEN** `spinloop remote deploy` runs with an spinloop version pin
- **THEN** the environment's stored deploy config carries that version, and
  the environment's next fresh boot installs exactly that release

#### Scenario: No pin leaves the boot on its default

- **WHEN** `spinloop remote deploy` runs without an spinloop version pin
- **THEN** the stored deploy config carries no spinloop version, and the
  environment's boots install the latest published release

#### Scenario: An empty pin value is ignored

- **WHEN** `spinloop remote deploy` is given an spinloop version pin whose value is
  empty or whitespace only
- **THEN** it is treated as if no pin were given

### Requirement: The deploy plan shows the resolved spinloop version

The plan `spinloop remote deploy` prints — including under `--dry-run`, before
any AWS work or send — SHALL state the spinloop version the environment's boots
will install: the pinned version when a pin is given, otherwise `latest`. It
SHALL appear alongside the runner and model the plan already prints.

#### Scenario: A pinned deploy prints the pinned version

- **WHEN** `spinloop remote deploy --dry-run` runs with an spinloop version pin
- **THEN** the printed plan names that pinned version as the spinloop the
  environment will run

#### Scenario: An unpinned deploy prints latest

- **WHEN** `spinloop remote deploy --dry-run` runs without an spinloop version pin
- **THEN** the printed plan names `latest` as the spinloop the environment will
  run
