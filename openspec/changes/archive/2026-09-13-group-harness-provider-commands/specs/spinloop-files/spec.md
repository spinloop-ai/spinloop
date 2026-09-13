# Delta: spinloop-files

## MODIFIED Requirements

### Requirement: Spinloop path resolution

Commands that take a Spinloop path (`apply`, `unapply`, `serve`, `alias`,
`harness --spinloop`, and the `remote` subcommands) SHALL default to `./Spinloop`
when no path is given, SHALL accept a directory and use the `Spinloop` file
inside it, SHALL accept a registered alias name in place of a path, and SHALL
accept an `http://` or `https://` URL in place of a path, fetched over HTTP
instead of read from local disk. A URL ending in `/` SHALL be treated as a
directory-style reference and have `Spinloop` appended, mirroring the local
directory case. When the default `./Spinloop` is missing, the error SHALL
suggest passing a path or an alias.

When no path is given, the `SPINLOOP_ALIAS` environment variable SHALL be
consulted before falling back to `./Spinloop`, so the resolution order is the
argument, then `SPINLOOP_ALIAS`, then `./Spinloop`. `spinloop alias` SHALL be the one
exception and always use its argument or `./Spinloop`. Where the default
`./Spinloop` is missing and no `SPINLOOP_ALIAS` is set, the error SHALL name the
variable alongside the path and alias it already suggests.

#### Scenario: Bare command in a project directory

- **WHEN** the user runs `spinloop harness apply` in a directory holding a `Spinloop`
- **THEN** that file is applied

#### Scenario: Directory argument

- **WHEN** the user runs `spinloop harness apply path/to/dir` and the directory holds an
  `Spinloop`
- **THEN** `path/to/dir/Spinloop` is applied

#### Scenario: A remote subcommand resolves the same way

- **WHEN** the user runs `spinloop remote status` in a directory holding an
  `Spinloop`
- **THEN** that Spinloop is read to find the endpoint's configuration

#### Scenario: The environment names the default Spinloop

- **WHEN** `SPINLOOP_ALIAS` names a registered alias and the user runs
  `spinloop serve` with no argument
- **THEN** that alias's Spinloop is served, whether or not the working directory
  holds one

#### Scenario: Nothing to resolve

- **WHEN** the user runs `spinloop harness apply` with no argument, no `SPINLOOP_ALIAS` set
  and no `./Spinloop` present
- **THEN** the command fails suggesting a path, an alias, or `SPINLOOP_ALIAS`

#### Scenario: A URL argument

- **WHEN** the user runs `spinloop harness apply https://example.com/team/Spinloop`
- **THEN** the Spinloop is fetched from that URL and applied, with no local
  file read

#### Scenario: A directory-style URL argument

- **WHEN** the user runs `spinloop harness apply https://example.com/team/` (a
  trailing `/`)
- **THEN** `https://example.com/team/Spinloop` is fetched and applied

#### Scenario: An unreachable URL

- **WHEN** the user runs `spinloop harness apply` against a URL whose host does not
  respond
- **THEN** the command fails with a clear network error naming the URL,
  rather than a filesystem "not found" error

### Requirement: Applying and unapplying a Spinloop

`spinloop harness apply` SHALL apply the Spinloop's selection exactly as the equivalent
`spinloop harness add` would, and `spinloop harness unapply` SHALL remove what the Spinloop selects
exactly as the equivalent `spinloop harness remove` would. A command-line `--output`/`-o`
on `apply` SHALL override the Spinloop's `OUTPUT` instruction, and `--providers`
SHALL override the catalogue it resolves against (a Spinloop never names a
catalogue). `apply` SHALL ignore a `PRESET` instruction — it is consumed only
by `spinloop serve`.

#### Scenario: Apply equals add

- **WHEN** a Spinloop with `PROVIDER ollama` and `MODEL llama3.2` is applied
- **THEN** the harness config matches what `spinloop harness add -p ollama -m llama3.2`
  would have produced

#### Scenario: Output override

- **WHEN** a Spinloop sets `OUTPUT 32k` and the user runs
  `spinloop harness apply --output 16k`
- **THEN** the applied output limit is 16000 tokens

#### Scenario: Preset is not apply's business

- **WHEN** a Spinloop with a `PRESET` instruction is applied
- **THEN** the harness config is written as if the instruction were absent

### Requirement: Exporting the current config

`spinloop harness export` SHALL reconstruct a canonical Spinloop from the active harness's
config and print it to stdout. The provider exported is chosen by the
`--provider`/`-p` flag, else the default model's provider, else the sole
configured provider; with several providers and no way to choose, the command
SHALL fail listing them. The output SHALL name the configured model with a
`MODEL` instruction, SHALL omit a `BASEURL` that only restates the catalogue's
default, and SHALL record `CONTEXT`/`OUTPUT` only when the exported models agree
on a single value — never inventing one. Rendered output SHALL use canonical
UPPERCASE keywords with aligned values, so `spinloop harness export > Spinloop`
round-trips.

#### Scenario: Round-trip through export

- **WHEN** the user applies a Spinloop and then runs `spinloop harness export`
- **THEN** the printed Spinloop selects the same provider, model, and limits

#### Scenario: Ambiguous provider

- **WHEN** several providers are configured, none is the default model's, and
  no `-p` is given
- **THEN** the command fails listing the configured providers to choose from

#### Scenario: Nothing configured

- **WHEN** the harness config has no providers
- **THEN** the command fails naming the config file it read
