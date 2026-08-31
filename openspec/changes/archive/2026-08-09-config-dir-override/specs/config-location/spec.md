## Purpose

Define where spinloop keeps its own machine-local state — the single config
directory under which `config.json`, `remote.json`, the environment registry,
the daemon state, and the CDK source all live — and how that directory is
resolved and overridden, so a service (not just an interactive user) can be
told exactly where its config is.

## ADDED Requirements

### Requirement: Single resolved config directory

spinloop SHALL resolve one config directory and place every file it owns under
it: its own `config.json` (default-harness preference and alias registry), the
legacy `remote.json`, the `remotes/<name>/` environment registry, the daemon
state directory, and the CDK source directory. There SHALL be one resolver;
the location SHALL NOT be computed independently in more than one place.

#### Scenario: All spinloop-owned state shares one root

- **WHEN** the config directory resolves to a given path
- **THEN** `config.json`, `remote.json`, the `remotes/<name>/` registry, and
  the daemon state directory all resolve beneath that same path

### Requirement: SPINLOOP_CONFIG_DIR override

When `SPINLOOP_CONFIG_DIR` is set, its value SHALL be spinloop's config directory
verbatim — used as-is, with no `spinloop` segment appended. It SHALL take
precedence over `XDG_CONFIG_HOME` and over the home-directory default.

#### Scenario: Override is used verbatim

- **WHEN** `SPINLOOP_CONFIG_DIR=/var/lib/spinloop` is set
- **THEN** spinloop's config directory is `/var/lib/spinloop` and, for example, the
  daemon reads its deploy config from `/var/lib/spinloop/daemon/`

#### Scenario: Override wins over XDG and home

- **WHEN** both `SPINLOOP_CONFIG_DIR` and `XDG_CONFIG_HOME` are set
- **THEN** the config directory is `SPINLOOP_CONFIG_DIR`'s value, ignoring
  `XDG_CONFIG_HOME`

### Requirement: Default resolution unchanged

With `SPINLOOP_CONFIG_DIR` unset, the config directory SHALL be
`${XDG_CONFIG_HOME}/spinloop` when `XDG_CONFIG_HOME` is set, otherwise
`~/.config/spinloop`. This is the existing behaviour and SHALL be preserved so
existing installs are unaffected.

#### Scenario: XDG default

- **WHEN** `SPINLOOP_CONFIG_DIR` is unset and `XDG_CONFIG_HOME` is set
- **THEN** the config directory is `${XDG_CONFIG_HOME}/spinloop`

#### Scenario: Home default

- **WHEN** neither `SPINLOOP_CONFIG_DIR` nor `XDG_CONFIG_HOME` is set and the
  home directory resolves
- **THEN** the config directory is `~/.config/spinloop`

### Requirement: Unresolvable home fails loudly

When `SPINLOOP_CONFIG_DIR` is unset, `XDG_CONFIG_HOME` is unset, and the home
directory cannot be resolved (no `$HOME`, as under a bare systemd service),
spinloop SHALL fail with an error that names `SPINLOOP_CONFIG_DIR` as the fix,
rather than silently resolving to a relative or root-anchored `.config`
directory.

#### Scenario: No home, no override

- **WHEN** none of `SPINLOOP_CONFIG_DIR`, `XDG_CONFIG_HOME`, or a resolvable
  home directory is available
- **THEN** the operation fails with an error naming `SPINLOOP_CONFIG_DIR`, and
  does not read or write a bogus relative `.config` path
