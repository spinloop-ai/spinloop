# Delta: model-discovery

## MODIFIED Requirements

### Requirement: Discovery is best-effort and quiet

Model discovery SHALL be best-effort. A network failure, a non-success HTTP status, a
timeout, a missing required key, or an unparseable response SHALL yield an empty model set
rather than an error, and SHALL NOT prevent the surrounding command from succeeding.
Discovery SHALL apply a bounded request timeout so a slow or unreachable endpoint cannot
hang a command.

#### Scenario: Offline discovery does not fail the command

- **WHEN** `spinloop provider list --models <provider>` runs and the provider's endpoint is
  unreachable
- **THEN** the command still prints the provider's plumbing, reports no models were found,
  and exits successfully

#### Scenario: A slow endpoint cannot hang the command

- **WHEN** a provider's endpoint does not respond within the discovery timeout
- **THEN** discovery abandons the request and returns no models

#### Scenario: Completion stays silent on discovery failure

- **WHEN** model completion sources from discovery and the endpoint errors
- **THEN** `__complete` offers no model candidates, exits zero, and writes nothing to
  stderr

### Requirement: Surfacing discovered models

`spinloop provider list --models <provider>` SHALL print the provider's discovered models
beneath its plumbing. Without `--models`, `spinloop provider list` SHALL behave as
before (plumbing only) and SHALL NOT perform any network request. Shell model
completion SHALL offer discovered models for a provider that supports discovery,
scoped to the `--provider` already on the line.

#### Scenario: Listing a provider's live models

- **WHEN** the user runs `spinloop provider list --models openrouter` and discovery succeeds
- **THEN** the provider's currently-served model ids are printed under its entry

#### Scenario: Plain list makes no network call

- **WHEN** the user runs `spinloop provider list` with no `--models` flag
- **THEN** no discovery request is made and only provider plumbing is printed

#### Scenario: Model completion offers discovered ids

- **WHEN** the user completes `spinloop harness add -p openrouter -m <TAB>` and
  discovery succeeds
- **THEN** the provider's discovered model ids are offered as candidates
