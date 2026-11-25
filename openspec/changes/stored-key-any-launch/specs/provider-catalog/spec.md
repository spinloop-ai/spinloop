## MODIFIED Requirements

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
count as set: it is referenced in harness configs exactly as a set variable
is, and supplied to a launched harness under the key variable's name. The key
file SHALL NOT be read for a remote endpoint, and its contents SHALL NOT be
written to any harness config. No matching file, or an empty one, SHALL mean
the key is unset.

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
- **THEN** the harness config references the key variable as it would for a
  set variable, and a harness launched by spinloop has the variable set to the
  file's contents with surrounding whitespace removed

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
