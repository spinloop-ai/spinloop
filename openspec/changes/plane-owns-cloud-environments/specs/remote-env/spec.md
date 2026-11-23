# Spec Delta

## MODIFIED Requirements

### Requirement: cloud env subcommand

The CLI SHALL provide `spinloop cloud env` as a subcommand that returns the
cloud endpoint's environment variables without booting the instance.

It SHALL select which environment it reads using the same rule as the other
`cloud` subcommands: the `--node <name>` flag naming an environment on the
resolved plane, with no fallback when the flag is absent. A Spinloop given as an
argument SHALL be read only for its `ENV` instructions and adjacent `.env`,
never to select an environment.

Its stdout SHALL carry nothing but the `export` lines, so
`eval "$(spinloop cloud env)"` is safe in a shell. Anything the command has to
say for itself — notably the note reporting which alias resolved the Spinloop —
SHALL go to stderr, where it is still visible in a terminal but outside what the
shell evaluates.

#### Scenario: env returns exports for a running endpoint

- **WHEN** the user runs `spinloop cloud env --node prod` and the environment is
  deployed
- **THEN** stdout contains `export OPENAI_BASE_URL=<url>` and
  `export OPENAI_API_KEY=<key>`, read from the plane without starting anything

#### Scenario: env fails when endpoint is stopped

- **WHEN** the user runs `spinloop cloud env --node prod` and the plane holds no
  environment `prod`
- **THEN** the command fails saying the plane holds no such environment and
  naming the deploy command that would create one

#### Scenario: env names the environment with the flag

- **WHEN** the user runs `spinloop cloud env --node dev-2`
- **THEN** the command resolves the plane from its credentials and region, reads
  the environment `dev-2` there, and reports on that environment's endpoint

#### Scenario: env falls back to the default environment

- **WHEN** the user runs `spinloop cloud env` with no `--node` flag and the plane
  holds an environment named `default`
- **THEN** no fallback happens: the command prints no exports, fails naming the
  flag, and does not assume `default`

#### Scenario: env outputs nothing to stderr on success

- **WHEN** the user runs `spinloop cloud env --node prod` with no alias to
  report, and the environment is deployed
- **THEN** stderr is empty (only export lines on stdout)

#### Scenario: env stdout is eval-safe for an aliased Spinloop

- **WHEN** the user runs `eval "$(spinloop cloud env <alias>)"` with `--node`
  named and the environment deployed
- **THEN** every line on stdout is an `export` line, the alias note having gone
  to stderr, and the shell evaluates it without error
