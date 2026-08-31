## ADDED Requirements

### Requirement: remote env command exists
The CLI SHALL provide `spinloop remote env` as a subcommand that returns the remote endpoint's environment variables from an already-running instance.

#### Scenario: env returns exports for a running endpoint
- **WHEN** the user runs `spinloop remote env` and the remote instance is running
- **THEN** stdout contains `export OPENAI_BASE_URL=<url>` and `export OPENAI_API_KEY=<key>`

#### Scenario: env fails when endpoint is stopped
- **WHEN** the user runs `spinloop remote env` and the remote instance is stopped
- **THEN** the command fails with an error telling the user to run `spinloop remote start` first

#### Scenario: env resolves Spinloop via REMOTE instruction
- **WHEN** the user runs `spinloop remote env ./Spinloop` and the Spinloop contains a `REMOTE` instruction
- **THEN** the command resolves the remote config from the `REMOTE` value relative to the Spinloop's directory

#### Scenario: env fallback to default Spinloop
- **WHEN** the user runs `spinloop remote env` with no arguments in a directory containing an `Spinloop` file with a `REMOTE` instruction
- **THEN** the command uses that Spinloop's remote config

#### Scenario: env fallback to per-user config
- **WHEN** the user runs `spinloop remote env` with no arguments and no `./Spinloop` is present (or it has no `REMOTE`)
- **THEN** the command uses the per-user remote config

#### Scenario: env outputs nothing to stderr on success
- **WHEN** the user runs `spinloop remote env` and the endpoint is running
- **THEN** stderr is empty (only export lines on stdout)

### Requirement: env Lambda is fast (no boot)
The `spinloop remote env` command SHALL NOT trigger an instance boot. It only reads the API key from Secrets Manager and the base URL from the environment's Elastic IP.

#### Scenario: env does not start a stopped instance
- **WHEN** the user runs `spinloop remote env` and the instance is stopped
- **THEN** the command returns quickly with an error (not after minutes of booting)

### Requirement: start -e/--env flag prints exports
The `spinloop remote start` command SHALL accept `-e` and `--env` flags that, when present, print the export lines after a successful start.

#### Scenario: start with --env prints exports
- **WHEN** the user runs `spinloop remote start --env` and the instance starts successfully
- **THEN** stdout contains the `export OPENAI_BASE_URL` and `export OPENAI_API_KEY` lines

#### Scenario: start without flag suppresses exports
- **WHEN** the user runs `spinloop remote start` without `-e` or `--env`
- **THEN** stdout does not contain export lines (only stderr progress)

### Requirement: start default behaviour changed
By default (no flags), `spinloop remote start` SHALL NOT print export lines to stdout.

#### Scenario: bare start produces no stdout exports
- **WHEN** the user runs `spinloop remote start` and the instance starts successfully
- **THEN** stdout is empty (progress goes to stderr only)
