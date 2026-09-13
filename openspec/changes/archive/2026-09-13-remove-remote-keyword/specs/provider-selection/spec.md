# Delta: provider-selection

## ADDED Requirements

### Requirement: Base URL resolution

The system SHALL let the user override any provider's API base URL, resolved
with the precedence: the explicit override, then the `SPINLOOP_BASE_URL`
environment variable, then the catalogue's per-provider values. The explicit
override is the `--base-url`/`-u` flag, or the `BASEURL` of the Spinloop being
applied.

When `apply` is given an `--env <name>` flag and the Spinloop states no
`BASEURL` of its own, applying it SHALL take the base URL from the named
environment's registered configuration, so the endpoint's address stays with
the deployment that owns it rather than in the hand-written Spinloop. The
system SHALL report that it did so. When the Spinloop states a `BASEURL` of its
own, that value SHALL be the applied base URL and the environment's own base
URL SHALL NOT be read; the environment still supplies the provider's name and,
for a launch, its key.

#### Scenario: Flag beats environment and catalogue

- **WHEN** `--base-url https://gateway/v1` is given and `SPINLOOP_BASE_URL` is
  also set
- **THEN** the configured base URL is `https://gateway/v1`

#### Scenario: Base URL from the named environment

- **WHEN** a Spinloop with no `BASEURL` is applied with `--env dev-1`, and that
  environment's registered configuration names a base URL
- **THEN** the provider is configured with the base URL from that
  configuration, and the output says where it came from

#### Scenario: A Spinloop's own BASEURL wins

- **WHEN** a Spinloop states a `BASEURL` and is applied with `--env dev-1`
  whose configuration names a different base URL
- **THEN** the provider is configured with the Spinloop's `BASEURL`, and the
  provider is still keyed on the environment `dev-1`

## REMOVED Requirements

### Requirement: Base URL precedence

**Reason**: The base-URL fallback read the configuration a Spinloop's `REMOTE`
named, including a path/URL form that is no longer addressable, and tolerated a
not-yet-written configuration; with the `--env` flag the environment is an
explicit name, so a missing registration is reported rather than waited for.

**Migration**: Apply with `--env <name>`; the base URL comes from that
environment's registered configuration, and an unregistered name fails saying
to deploy it.
