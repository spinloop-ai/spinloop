## MODIFIED Requirements

### Requirement: Remote configuration discovery

The endpoint's control URLs SHALL come from a JSON configuration naming a start
URL, a stop URL, an optional deploy URL, and a region. That configuration MAY
also name the endpoint's own base URL; it SHALL be optional, since no control
call needs it, and a configuration without it SHALL remain valid. A Spinloop's
`REMOTE` instruction SHALL select that configuration: a bare name selects the
named environment from the per-user registry (always local; see the Remote
Environments specification), and a path or URL selects a configuration
resolved relative to the Spinloop's own source when not itself absolute — a
local directory join when the Spinloop was read from disk, URL-relative
resolution when the Spinloop was fetched from a URL — and fetched over HTTP when
it resolves to a URL. Fetching a remote `REMOTE` configuration SHALL happen
only at the point a `remote` subcommand, or `spinloop apply`'s base-URL
fallback, actually resolves it. When no Spinloop names one, the `default`
environment SHALL be used, so the command works outside any project.
Environment variables SHALL override individual values, and the region SHALL
fall back to the standard AWS region variable and then to the region named in
the URL. A missing or incomplete configuration SHALL fail saying where to put
it.

#### Scenario: Spinloop names the configuration

- **WHEN** a Spinloop sets `REMOTE ./remote.json` and a `remote` subcommand
  runs with that Spinloop
- **THEN** the URLs come from that file, resolved beside the Spinloop

#### Scenario: Spinloop names an environment

- **WHEN** a Spinloop sets `REMOTE qwen3.6-27b-prod` and a `remote` subcommand
  runs with that Spinloop
- **THEN** the URLs come from that environment's `remote.json` in the registry

#### Scenario: Explicit Spinloop without a REMOTE instruction

- **WHEN** a `remote` subcommand is given a Spinloop that has no `REMOTE`
- **THEN** it fails saying that Spinloop has no `REMOTE` instruction, rather than
  silently using the default environment

#### Scenario: No Spinloop in play

- **WHEN** a `remote` subcommand runs outside a project
- **THEN** the `default` environment is used

#### Scenario: Configuration without a base URL

- **WHEN** a remote configuration names the control URLs and region but no base
  URL, and a `remote` subcommand runs
- **THEN** the subcommand works as it always has, since the endpoint reports its
  own address in the replies to `start` and `status`

#### Scenario: A remote configuration fetched over HTTP

- **WHEN** a Spinloop sets `REMOTE https://example.com/team/remote.json`
- **THEN** a `remote` subcommand fetches that URL for the control
  configuration

#### Scenario: A REMOTE relative to a URL-sourced Spinloop

- **WHEN** a Spinloop fetched from `https://example.com/team/Spinloop` sets
  `REMOTE ./remote.json`
- **THEN** the configuration resolves to
  `https://example.com/team/remote.json` and is fetched

#### Scenario: A remote REMOTE is fetched only by commands that resolve one

- **WHEN** `spinloop serve` runs against a Spinloop whose `REMOTE` is a URL
- **THEN** the `REMOTE` URL is never fetched — `serve` has no use for a
  remote endpoint's control configuration

#### Scenario: Applying names the environment even with an explicit BASEURL

- **WHEN** a Spinloop with a URL-form `REMOTE` and its own `BASEURL`
  instruction is applied with `spinloop apply`
- **THEN** the `REMOTE` URL is still fetched once, to name the harness
  provider after the deployment's environment (the same read a local-path
  `REMOTE` already triggers) — only the redundant base-URL lookup is skipped,
  since `BASEURL` already supplies it
