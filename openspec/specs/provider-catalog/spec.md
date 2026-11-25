# Provider Catalog Specification

## Purpose

Define the catalogue of model providers that `spinloop` can configure: where the
catalogue comes from, what it declares (provider connection plumbing, not
models), how users inspect it (`spinloop provider list`), and how they replace it
with their own (`spinloop provider init`, `--providers`, `SPINLOOP_PROVIDERS`).
## Requirements

### Requirement: Provider command group

A `spinloop provider` group SHALL hold the catalogue commands: `list` and
`init`, each carrying the flags, arguments, and output its former top-level
spelling had (the old `init-providers` is renamed `init`, since the parent
already says provider). A bare `spinloop provider` SHALL show the group's help,
as the `fleet` and `cloud` groups do.

The former top-level spellings of the two catalogue commands SHALL
NOT exist. Invoking one SHALL fail with an error that names the command's new
home — giving `spinloop provider list` for `list` and `spinloop provider init`
for `init-providers` — rather than the generic unknown-command message.

#### Scenario: A subcommand runs under provider

- **WHEN** the user runs `spinloop provider list`
- **THEN** the catalogue is printed exactly as the former top-level `list` did
  before the move

#### Scenario: The old top-level spelling names its new home

- **WHEN** the user runs the former top-level spelling `list`
- **THEN** the command fails, saying list has moved under provider and giving
  `spinloop provider list`

#### Scenario: Bare provider shows its help

- **WHEN** the user runs `spinloop provider` with no argument
- **THEN** the group's help is shown, naming `list` and `init`

### Requirement: Catalogue listing

`spinloop provider list` SHALL print every provider in the catalogue in stable
(alphabetical) order, showing for each: its id and description, its API key
environment variable (marked `(required)` when the key is mandatory), and the
harnesses that support it (`opencode`, plus `pi` when the provider has a `pi`
block).

#### Scenario: Listing the built-in catalogue

- **WHEN** the user runs `spinloop provider list`
- **THEN** every embedded provider is printed with its key requirements and
  supported harnesses

### Requirement: Runtime catalogue override

The system SHALL let the user substitute their own catalogue file at runtime,
resolved with the precedence: `--providers` flag, then the `SPINLOOP_PROVIDERS`
environment variable, then the embedded catalogue.

#### Scenario: Flag wins over environment variable

- **WHEN** both `--providers ./mine.yaml` and `SPINLOOP_PROVIDERS=./other.yaml`
  are set
- **THEN** the catalogue is loaded from `./mine.yaml`

#### Scenario: Unreadable override is an error

- **WHEN** the resolved catalogue path cannot be read or parsed as YAML
- **THEN** the command fails with an error naming the file

### Requirement: Catalogue scaffolding

`spinloop provider init [path]` SHALL write a copy of the embedded
catalogue to `./providers.yaml` (or the given path) as a starting point for
customisation, and SHALL refuse to overwrite an existing file unless
`--force`/`-F` is given. On success it SHALL print how to point `spinloop` at
the written file.

#### Scenario: Refuses to clobber an existing file

- **WHEN** the user runs `spinloop provider init` and `./providers.yaml`
  already exists
- **THEN** the command fails, telling the user to pass a different path or
  `--force`

#### Scenario: Writing the catalogue out

- **WHEN** the user runs `spinloop provider init custom.yaml` and no such
  file exists
- **THEN** the embedded catalogue is written to `custom.yaml` byte-for-byte

### Requirement: Embedded provider definitions

The system SHALL ship with a built-in catalogue of providers, defined in a YAML
file (`providers.yaml`) embedded into the binary at build time. Each provider
entry declares a description and MAY declare a display name, an npm package, an
API key environment variable (with optional required flag, optional flag,
expected key prefix, and key file), static options (such as `baseURL`), options
resolved from environment variables, a list of option keys that are required, a
`pi` block marking the provider as usable by the Pi harness, and a `lucinate`
marker identifying the provider as usable by the lucinate harness. The
`lucinate` marker SHALL be present only on providers that expose an
OpenAI-compatible endpoint; a provider without it SHALL be reported as
unsupported by the lucinate harness. The catalogue SHALL NOT enumerate models:
the model a provider serves is named by the user's selection, not stored in the
catalogue.

A provider whose API key is declared optional is one that also works
unauthenticated — the same engine run as a local server and as an
authenticated cloud endpoint — so an unset key variable SHALL mean "no key",
not "a key that is missing".

A provider MAY declare `apiKeyFile`: a path pattern naming a file in which a
local server stores the key it generated. The pattern MAY contain environment
references such as `$HOME`, which SHALL be expanded, and glob wildcards. When
the provider's key variable is set nowhere spinloop looks (an `ENV`
instruction, the process environment, or the `.env` beside the Spinloop), and
the provider's endpoint is local, the key SHALL be read from the file the
pattern matches, with surrounding whitespace removed; when several files
match, the most recently modified one SHALL be used. A key read this way SHALL
count as set, and SHALL be supplied to a launched harness under the key
variable's name. In the opencode config, the provider's key SHALL reference
the matched file by its path, in opencode's file-reference form, so that any
opencode process that loads the config reads the key itself, whatever its own
environment. The other harnesses SHALL reference the key variable, as they do
for a set variable. The key file SHALL NOT be read for a remote endpoint, and
its contents SHALL NOT be written to any harness config. No matching file, or
an empty one, SHALL mean the key is unset.

Whether a launch looks up a stored key SHALL depend on the provider the
launched harness will use, not on whether the launch applies a Spinloop. That
provider SHALL be the applied Spinloop's when the launch applies one; otherwise
the provider of the harness's default model; and, for a harness whose config
names no default model, every provider its config holds. A provider whose id is
not in the catalogue SHALL get no stored key. The endpoint checked for being
local SHALL be the one the apply writes for an applied Spinloop; otherwise the
base URL the harness's config holds for the provider, or the catalogue's
endpoint when the config holds none. The provider of a launch through a cloud
endpoint or a fleet SHALL take its key from that endpoint, as before.

A provider MAY declare `optionsRequired`: a list of option keys that MUST resolve
to a non-empty value (whether from static options or from an environment-variable
mapping) when the provider is applied. When any listed key is missing or empty,
applying the provider SHALL fail with an error naming the option and the
environment variable that supplies it. This guards cloud providers that
authenticate via ambient credentials and so inject no API key, but still require
a caller-supplied value (such as a GCP project) that has no usable default.

#### Scenario: Catalogue loads without external files

- **WHEN** any command that needs the catalogue runs with no `--providers` flag
  and no `SPINLOOP_PROVIDERS` environment variable
- **THEN** the embedded catalogue is used, with no file read from disk

#### Scenario: An optional key is injected only when set

- **WHEN** a provider whose API key is optional is applied, with its key
  variable set
- **THEN** the key is injected into the harness config

#### Scenario: An optional key that is unset is not an error

- **WHEN** the same provider is applied with the key variable unset
- **THEN** the configuration is written with no key, and the command succeeds

#### Scenario: A key file supplies an unset key for a local endpoint

- **WHEN** a provider that declares `apiKeyFile` is applied against a local
  endpoint, its key variable is set nowhere, and a file matching the pattern
  holds a key
- **THEN** the opencode config references the matched file by its path, the
  Pi config references the key variable, and a harness launched by spinloop
  has the variable set to the file's contents with surrounding whitespace
  removed

#### Scenario: opencode reads the key file whatever its environment

- **WHEN** the opencode config has been applied with a key file reference, and
  opencode is started without the key variable in its environment, as a
  background service started earlier may be
- **THEN** opencode reads the key from the file and its requests are
  authenticated

#### Scenario: A key variable set at apply time is referenced, not the file

- **WHEN** the key variable is set when the opencode config is applied
- **THEN** the config references the key variable, and the key file is not
  named in it

#### Scenario: A set key variable takes precedence over the key file

- **WHEN** the key variable is set in an `ENV` instruction, the process
  environment, or the `.env` beside the Spinloop, and a key file also matches
- **THEN** the variable's value is used and the file is not read

#### Scenario: A launch with no Spinloop uses the default model's stored key

- **WHEN** a harness is launched without applying a Spinloop, its default
  model's provider declares `apiKeyFile`, the harness's config holds a local
  base URL for that provider, the provider's key variable is set nowhere, and
  a stored key exists
- **THEN** the launched harness has the key variable set to the stored key

#### Scenario: A launch with no Spinloop skips a provider configured remotely

- **WHEN** the harness's config holds that provider at a base URL that is not
  local
- **THEN** the launched harness does not get the stored key

#### Scenario: A harness with no default model gets each configured provider's key

- **WHEN** a harness whose config names no default model is launched without
  applying a Spinloop, and its config holds a provider that declares
  `apiKeyFile` at a local base URL
- **THEN** the launched harness gets that provider's stored key

#### Scenario: A provider the harness will not use gets no stored key

- **WHEN** a provider declares `apiKeyFile`, but the launch applies a
  Spinloop for a different provider, or the harness's default model belongs to
  a different provider
- **THEN** the launched harness does not get that provider's stored key

#### Scenario: The newest matching key file is used

- **WHEN** the pattern matches several files
- **THEN** the key is read from the most recently modified one

#### Scenario: A key file is not used for a remote endpoint

- **WHEN** a provider that declares `apiKeyFile` is applied against an endpoint
  that is not local, with its key variable unset
- **THEN** the key file is not read, and the key is treated as unset

#### Scenario: A missing key file leaves the key unset

- **WHEN** no file matches the pattern, or the matching file is empty
- **THEN** the key is treated as unset, and the command behaves as it does for
  a provider with no `apiKeyFile`

#### Scenario: A required option that is unset is an error

- **WHEN** a provider that declares a required option is applied and neither the
  option's static value nor its environment-variable source resolves to a
  non-empty value
- **THEN** the command fails with an error naming the option and its
  environment variable, and no config is written

#### Scenario: A required option resolved from the environment succeeds

- **WHEN** a provider that declares a required option is applied with that
  option's environment variable set to a non-empty value
- **THEN** the option is injected into the harness config and the command
  succeeds

#### Scenario: An OpenAI-compatible provider is lucinate-capable

- **WHEN** a provider that declares the `lucinate` marker is applied under the
  lucinate harness
- **THEN** the provider is accepted and its connection is written

#### Scenario: A provider without the marker is unsupported by lucinate

- **WHEN** a provider that does not declare the `lucinate` marker is applied
  under the lucinate harness
- **THEN** the command fails reporting the provider unsupported by lucinate

### Requirement: Vertex AI providers

The built-in catalogue SHALL include two Google Cloud Vertex AI providers,
keyed by the provider ids opencode resolves natively (so neither declares an npm
package):

- `google-vertex` — Gemini models on Vertex AI.
- `google-vertex-anthropic` — Anthropic Claude models on Vertex AI.

Both SHALL authenticate via ambient Google credentials (Application Default
Credentials) and SHALL NOT inject an API key, mirroring `amazon-bedrock`. Both
SHALL declare a `project` option (required, resolved from `GOOGLE_VERTEX_PROJECT`)
and a `location` option (resolved from `GOOGLE_VERTEX_LOCATION`, defaulting to
`global`). Neither SHALL declare a `pi` block: Vertex is not supported by the Pi
harness.

#### Scenario: Applying a Vertex provider with a project set

- **WHEN** the user applies `google-vertex-anthropic` (or `google-vertex`) with
  `GOOGLE_VERTEX_PROJECT` set to a non-empty value
- **THEN** an opencode provider block is written with the `project` and
  `location` options and no `apiKey`, and the command succeeds

#### Scenario: Applying a Vertex provider without a project fails

- **WHEN** the user applies a Vertex provider with `GOOGLE_VERTEX_PROJECT` unset
  and no `project` override
- **THEN** the command fails with an error naming the `project` option and
  `GOOGLE_VERTEX_PROJECT`

#### Scenario: Vertex is not offered to the Pi harness

- **WHEN** the user targets the Pi harness with a Vertex provider
- **THEN** the command reports the provider as unsupported by the Pi harness

### Requirement: Apple Silicon local provider

The catalogue SHALL include an `omlx` provider describing a local
[oMLX](https://omlx.ai) server: an OpenAI-compatible endpoint defaulting to
`http://localhost:8000/v1`, overridable per-provider with `OMLX_BASE_URL`, and
Pi-capable through the `openai-completions` API.

Like the llama.cpp provider it SHALL be optionally authenticated: a local server
needs no key, while the same provider may name an oMLX server started with
`--api-key` on another machine. An unset key at a local endpoint therefore
yields no `apiKey` option for opencode and the keyless placeholder for Pi, while
a set key — or any non-local endpoint — yields the environment reference the
harness resolves at run time.

Its default port coinciding with another provider's is not a conflict: the
catalogue places no uniqueness requirement on base URLs.

#### Scenario: Local server needs no key

- **WHEN** a selection names `omlx` with the default localhost base URL and no
  API key set
- **THEN** the opencode provider block carries no `apiKey` option, and the Pi
  entry carries the keyless placeholder so its models stay selectable

#### Scenario: Remote server keeps the key reference

- **WHEN** the base URL is overridden to a non-local host
- **THEN** the API key is written as an environment reference, never as the
  resolved secret

### Requirement: MTPLX local provider

The catalogue SHALL include an `mtplx` provider describing a local
[MTPLX](https://mtplx.com) server: an OpenAI-compatible endpoint defaulting to
`http://localhost:8000/v1`, overridable per-provider with `MTPLX_BASE_URL`, and
Pi-capable through the `openai-completions` API.

Like the llama.cpp and oMLX providers it SHALL be optionally authenticated: a
local server needs no key, while the same provider may name an MTPLX server
started with `--api-key` on another machine. An unset key at a local endpoint
therefore yields no `apiKey` option for opencode and the keyless placeholder
for Pi, while a set key — or any non-local endpoint — yields the environment
reference the harness resolves at run time.

Its default port coinciding with another provider's is not a conflict: the
catalogue places no uniqueness requirement on base URLs.

#### Scenario: Local server needs no key

- **WHEN** a selection names `mtplx` with the default localhost base URL and no
  API key set
- **THEN** the opencode provider block carries no `apiKey` option, and the Pi
  entry carries the keyless placeholder so its models stay selectable

#### Scenario: Remote server keeps the key reference

- **WHEN** the base URL is overridden to a non-local host
- **THEN** the API key is written as an environment reference, never as the
  resolved secret

### Requirement: Unsloth Studio local provider

The catalogue SHALL include an `unsloth` provider describing a local
[Unsloth Studio](https://unsloth.ai) server: an OpenAI-compatible endpoint
defaulting to `http://localhost:8888/v1`, overridable per-provider with
`UNSLOTH_BASE_URL`, and Pi-capable through the `openai-completions` API.

Its key SHALL be read from `UNSLOTH_API_KEY` and SHALL be optional: a local
endpoint with the key unset yields no `apiKey` option for opencode and the
keyless placeholder for Pi.

Studio generates its key itself and stores it for reuse, so the provider SHALL
declare as its key file the file `unsloth run` stores its key in under the
default key name (`$HOME/.unsloth/studio/auth/.cli_api_key_cli_*`). A local
Studio's key SHALL then be found without the user exporting it.

#### Scenario: Local server with keyless access needs no key

- **WHEN** a selection names `unsloth` with the default localhost base URL,
  `UNSLOTH_API_KEY` unset, and no stored Studio key
- **THEN** the opencode provider block carries no `apiKey` option, and the Pi
  entry carries the keyless placeholder so its models stay selectable

#### Scenario: Studio's stored key is used for a local server

- **WHEN** a selection names `unsloth` with a local base URL,
  `UNSLOTH_API_KEY` is set nowhere, and Studio has stored a key
- **THEN** the opencode config references Studio's key file by its path, the
  Pi config references `UNSLOTH_API_KEY`, and a harness launched by spinloop
  has `UNSLOTH_API_KEY` set to the stored key

#### Scenario: The Studio key is referenced, never written

- **WHEN** `UNSLOTH_API_KEY` is set, or the key comes from Studio's stored key
- **THEN** the API key is written as a reference that the harness resolves at
  run time (to `UNSLOTH_API_KEY`, or for opencode with the variable unset, to
  Studio's key file), never as the resolved secret

#### Scenario: A remote Studio keeps the key reference

- **WHEN** the base URL is overridden with `UNSLOTH_BASE_URL` to a non-local
  host
- **THEN** the API key is written as an environment reference, never as the
  resolved secret, and Studio's locally stored key is not used
