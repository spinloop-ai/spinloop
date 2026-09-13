# Delta: remote-env

## ADDED Requirements

### Requirement: remote env subcommand
The CLI SHALL provide `spinloop remote env` as a subcommand that returns the remote endpoint's environment variables from an already-running instance.

It SHALL select which environment it reads using the same rule as the other
`remote` subcommands: the `--env <name>` flag naming a registered environment,
falling back to the `default` environment when the flag is absent. A Spinloop
given as an argument SHALL be read only for its `ENV` instructions and adjacent
`.env`, never to select an environment.

Its stdout SHALL carry nothing but the `export` lines, so `eval "$(spinloop remote env)"` is safe in a shell. Anything the command has to say for itself — notably the note reporting which alias resolved the Spinloop — SHALL go to stderr, where it is still visible in a terminal but outside what the shell evaluates.

#### Scenario: env returns exports for a running endpoint
- **WHEN** the user runs `spinloop remote env` and the remote instance is running
- **THEN** stdout contains `export OPENAI_BASE_URL=<url>` and `export OPENAI_API_KEY=<key>`

#### Scenario: env fails when endpoint is stopped
- **WHEN** the user runs `spinloop remote env` and the remote instance is stopped
- **THEN** the command fails with an error telling the user to run `spinloop remote start` first

#### Scenario: env names the environment with the flag
- **WHEN** the user runs `spinloop remote env --env dev-2`
- **THEN** the command reads `dev-2`'s control configuration from the registry
  and reports on that environment's endpoint

#### Scenario: env falls back to the default environment
- **WHEN** the user runs `spinloop remote env` with no `--env` flag
- **THEN** the command uses the `default` environment, whether or not a
  `./Spinloop` is present

#### Scenario: env outputs nothing to stderr on success
- **WHEN** the user runs `spinloop remote env` with no alias to report, and the endpoint is running
- **THEN** stderr is empty (only export lines on stdout)

#### Scenario: env stdout is eval-safe for an aliased Spinloop
- **WHEN** the user runs `eval "$(spinloop remote env <alias>)"` and the endpoint is running
- **THEN** every line on stdout is an `export` line, the alias note having gone to stderr, and the shell evaluates it without error

### Requirement: start --print-env flag prints exports

The `spinloop remote start` command SHALL accept a `--print-env` flag, with no
short form, that when present prints the export lines to stdout after a
successful start. The flag's former `-e`/`--env` spelling is removed, since
`--env` on `start` now selects the environment like on every other `remote`
subcommand; the eval-safe export lines remain available from
`spinloop remote env`.

#### Scenario: start with --print-env prints exports

- **WHEN** the user runs `spinloop remote start --print-env` and the instance
  starts successfully
- **THEN** stdout contains the `export OPENAI_BASE_URL` and
  `export OPENAI_API_KEY` lines

#### Scenario: start without the flag suppresses exports

- **WHEN** the user runs `spinloop remote start` without `--print-env`
- **THEN** stdout does not contain export lines (only stderr progress)

## REMOVED Requirements

### Requirement: remote env command exists

**Reason**: The `REMOTE`-based resolution scenarios (resolving via the
Spinloop's `REMOTE` instruction, falling back to the default Spinloop, then to
the per-user config) describe selection that the `--env` flag replaces.

**Migration**: Use `spinloop remote env --env <name>`; with no flag the
`default` environment is used.

### Requirement: start -e/--env flag prints exports

**Reason**: `-e`/`--env` on `start` is taken over by the environment-selection
flag the whole `remote` group now carries, so the export-printing flag needs a
new spelling.

**Migration**: Use `spinloop remote start --print-env` (no short form) for the
same behaviour, or `spinloop remote env`, whose stdout is eval-safe by design.
