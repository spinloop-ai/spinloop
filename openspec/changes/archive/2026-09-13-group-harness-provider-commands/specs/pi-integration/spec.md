# Delta: pi-integration

## MODIFIED Requirements

### Requirement: Config location and shape

The Pi adapter SHALL write `~/.pi/agent/models.json` — resolved from the home
directory, not XDG — as plain JSON of the form
`{"providers": {"<id>": {baseUrl, api, apiKey, models: [...]}}}`. A missing
file SHALL be treated as an empty document, and the directory SHALL be created
when needed. The file SHALL be written with owner-only (`0600`) permissions.

#### Scenario: First write creates the file

- **WHEN** the user runs `spinloop harness add -H pi` and no `models.json` exists
- **THEN** the file is created with the managed provider entry and owner-only
  permissions

### Requirement: Preserving merge

Writes SHALL merge only the managed provider entry: unknown top-level keys,
sibling providers, and unknown fields on the managed provider (headers,
compat, model overrides, …) SHALL all round-trip untouched. The provider's
scalar fields (`baseUrl`, `api`, `apiKey`) are overwritten when set; the
`models` arrays are unioned by `id` with incoming entries winning, and written
sorted by id so the file is deterministic.

#### Scenario: Sibling provider and unknown fields survive

- **WHEN** `models.json` holds another provider and extra fields on the managed
  one, and the user re-runs `spinloop harness add -H pi`
- **THEN** the sibling provider and the extra fields are intact afterwards

#### Scenario: Models unioned by id

- **WHEN** the managed provider already lists a model the selection also names
- **THEN** the incoming entry replaces it and no duplicate id appears

### Requirement: No default model on Pi

Pi has no default-model setting, so applying a selection SHALL NOT record one;
instead the command SHALL tell the user which model to select with `/model`
inside Pi. `spinloop harness export` for Pi SHALL rely on the provider
selection alone.

#### Scenario: Add tells the user what to pick

- **WHEN** a selection with a chosen model is applied to Pi
- **THEN** the output notes that Pi has no default-model setting and names the
  model to pick with `/model`

### Requirement: Per-model limits

When the selection carries a context window and output limit, they SHALL be
written on every model the selection adds, as `contextWindow` and `maxTokens`.

#### Scenario: Limits land on the Pi models

- **WHEN** `spinloop harness add -H pi -p llamacpp -m my-model -c 128k` is applied
- **THEN** the written model has `contextWindow` 128000 and `maxTokens` 32000
