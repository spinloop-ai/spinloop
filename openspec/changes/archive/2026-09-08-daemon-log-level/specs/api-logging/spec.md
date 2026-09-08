## MODIFIED Requirements

### Requirement: Records are graded by severity

Each record SHALL carry a severity so that turning the volume down silences
routine traffic first and failures last. A summary of a request that succeeded
SHALL be recorded at debug severity; a summary of a request rejected as the
caller's fault SHALL be recorded at warning severity; a summary of a request
that failed inside spinloop SHALL be recorded at error severity.

The consequence SHALL hold in both directions: an operator at the default
informational severity, or at warning severity, sees rejected and failed
requests and no successful ones, and an operator running at error severity sees
only spinloop's own failures.

#### Scenario: Routine traffic is silenced without silencing failures

- **WHEN** a fleet client polls status repeatedly at the default level, one of
  those requests carrying a bad token
- **THEN** no record is emitted for the successful polls
- **AND** the rejected request is still recorded

#### Scenario: A server-side failure is recorded at the highest severity

- **WHEN** a request fails with a server error
- **THEN** the summary is recorded at error severity, so it survives every
  level short of silence

### Requirement: The level is configurable

The host SHALL let an operator set the severity threshold at or above which
records are emitted, choosing between debug, informational, warning and error.
It SHALL be settable by a command-line flag on both `spinloop daemon` and
`spinloop serve`, and by an environment variable, with the flag taking
precedence over the variable. With neither set, the threshold SHALL be
informational — so rejected and failed requests, and the engine's starts and
stops, appear by default, and an operator who wants the routine request traffic
lowers the threshold to debug deliberately.

An unrecognised level SHALL be rejected at startup, naming the accepted values,
rather than being silently treated as the default: a mistyped level that
quietly logged everything anyway would be discovered only when the log was
needed.

#### Scenario: Summaries appear by default

- **WHEN** the API is exposed with no level configured, and one request is
  rejected as the caller's fault while another fails inside spinloop and a
  third is served successfully
- **THEN** a summary is emitted for the rejected request and for the failed
  one
- **AND** no summary is emitted for the successful one

#### Scenario: Raising the level silences summaries

- **WHEN** the level is set to warning
- **THEN** no summary is emitted for a successfully served request

#### Scenario: Debug brings summaries back

- **WHEN** the level is set to debug
- **THEN** a summary is emitted for a successfully served request

#### Scenario: The flag beats the environment

- **WHEN** the environment sets one level and the command line sets another
- **THEN** the command line's level applies

#### Scenario: An unrecognised level fails fast

- **WHEN** the configured level is not one of the accepted values
- **THEN** startup fails with an error naming the accepted values, and no API
  is exposed
