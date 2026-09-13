# remote-env Specification

## Purpose

How a caller gets the credentials for a running remote endpoint. The API key
is not in the local `remote.json` — it lives in AWS Secrets Manager, and the
base URL comes from the environment's Elastic IP — so it can only be fetched,
never read from disk. This covers `spinloop remote env`, which fetches it from an
already-running endpoint without booting one, and the `--print-env` flag that
opts `spinloop remote start` into printing the same exports.

## Requirements

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

### Requirement: env Lambda is fast (no boot)
The `spinloop remote env` command SHALL NOT trigger an instance boot. It only reads the API key from Secrets Manager and the base URL from the environment's Elastic IP.

#### Scenario: env does not start a stopped instance
- **WHEN** the user runs `spinloop remote env` and the instance is stopped
- **THEN** the command returns quickly with an error (not after minutes of booting)

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

### Requirement: start default behaviour changed
By default (no flags), `spinloop remote start` SHALL NOT print export lines to stdout.

#### Scenario: bare start produces no stdout exports
- **WHEN** the user runs `spinloop remote start` and the instance starts successfully
- **THEN** stdout is empty (progress goes to stderr only)
