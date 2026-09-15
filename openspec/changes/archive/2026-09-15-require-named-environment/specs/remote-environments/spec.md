## ADDED Requirements

### Requirement: Every environment resolves by name

An environment SHALL be resolved from its name and nothing else: the
configuration at `remotes/<name>/remote.json` in the per-user registry. No name
SHALL resolve by a different rule from any other, and no path outside the
registry SHALL be consulted, so a command that resolves an environment never
special-cases the name it was given.

Where a named environment has no registered file, a complete set of
`SPINLOOP_REMOTE_*` overrides SHALL configure it, so the remote commands can
run with nothing on disk. In that case the **name given** SHALL be the
environment identifier sent with each control call — the identifier the shared
lifecycle Lambdas use to tell one instance from another, which a configuration
assembled from variables alone has no other source for.

#### Scenario: A named environment reads its own file

- **WHEN** a command runs with `--env prod` and `remotes/prod/remote.json`
  exists
- **THEN** its configuration is read from that file

#### Scenario: A file at the superseded path is not read

- **WHEN** a command runs with `--env default`, no
  `remotes/default/remote.json` exists, and a file exists at the path the
  registry superseded
- **THEN** the command does not use that file's contents: no path outside the
  registry is consulted for any name

#### Scenario: Overrides configure a named environment with no file

- **WHEN** a command runs with `--env ci`, no `remotes/ci/remote.json` exists,
  and the `SPINLOOP_REMOTE_*` variables supply a complete configuration
- **THEN** the command works, and the control calls carry `ci` as the
  environment identifier

#### Scenario: Nothing anywhere fails naming the registry

- **WHEN** no configuration can be assembled for the named environment
- **THEN** the failure says the environment is not registered and how to
  create it
