## MODIFIED Requirements

### Requirement: remote env command exists
The CLI SHALL provide `spinloop remote env` as a subcommand that returns the remote endpoint's environment variables from an already-running instance.

Its stdout SHALL carry nothing but the `export` lines, so `eval "$(spinloop remote env)"` is safe in a shell. Anything the command has to say for itself — notably the note reporting which alias resolved the Spinloop — SHALL go to stderr, where it is still visible in a terminal but outside what the shell evaluates.

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
- **WHEN** the user runs `spinloop remote env` on a path, with no alias to report, and the endpoint is running
- **THEN** stderr is empty (only export lines on stdout)

#### Scenario: env stdout is eval-safe for an aliased Spinloop
- **WHEN** the user runs `eval "$(spinloop remote env <alias>)"` and the endpoint is running
- **THEN** every line on stdout is an `export` line, the alias note having gone to stderr, and the shell evaluates it without error
