## Why

The provider catalogue (`internal/catalog/providers.yaml`) doubles as a hand-curated
registry of example models, grouped into `families`. Models change constantly, so those
lists are an unbounded maintenance burden — yet they carry no context size, pricing, or
alias data, and `BuildProviderBlock` already synthesises a model entry for any id the
user names, so an arbitrary model already works without being listed. The `family` layer
exists only to bundle several models under one name; no shipped Spinloop uses `FAMILY`, and
the whole local-server story selects with `ALIAS`/`MODEL`. The curation earns nothing it
costs.

## What Changes

- **BREAKING**: Remove the `FAMILY` Spinloop keyword and the `--model-family`/`-f` flag. A
  selection is now a provider plus a `MODEL` and/or `ALIAS` (still at least one required).
- Collapse `providers.yaml` to provider *plumbing* only: description, name, npm, API-key
  env var (+ required/optional/prefix), options, optionsFromEnv, and the `pi` block. Drop
  every `families:` block, its `models`, and its per-family `defaultModel`.
- Remove the family machinery from the catalogue API: the `Family` type, `Provider.Families`,
  `MatchFamily`, `SortedFamilyNames`, and `Family.ModelKeys`; drop the `familyName` parameter
  from `BuildProviderBlock` and `BuildPiProvider`. The model comes solely from the user's
  `MODEL`/`ALIAS`.
- Simplify the commands that leaned on families: `spinloop list` no longer prints family
  lines (shows providers + auth/endpoint/harness support); `spinloop export` no longer names
  a family (it exports the configured model directly); `spinloop remove` drops family
  expansion. `spinloop init-providers` keeps dumping the (now smaller) embedded catalogue.
- Update shell completion to stop offering family names and to complete models without
  family scoping.

Non-goal: this change does not add any live/dynamic model source. Losing the browsable
model list from `spinloop list` is intentional here and is addressed separately.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `provider-catalog`: providers no longer declare model families or a family default
  model; the embedded catalogue is provider plumbing only, and `spinloop list` no longer
  lists families/models.
- `provider-selection`: a selection is validated as a provider plus a model and/or alias
  (no family); the "family expansion and default model" behaviour is removed, and removal
  no longer expands a family.
- `spinloop-files`: the `FAMILY` keyword is removed from the Spinloop format, and `export` no
  longer emits `FAMILY`.
- `shell-completion`: completion no longer offers family names, and model completion is no
  longer scoped by `--model-family`.

## Impact

- Data: `internal/catalog/providers.yaml`.
- Code: `internal/catalog/catalog.go`, `internal/spinloop/spinloop.go`,
  `internal/harness/adapters.go`, `cmd/spinloop/main.go`, `cmd/spinloop/complete.go`.
- Tests: `internal/catalog/catalog_test.go`, `internal/spinloop/spinloop_test.go`,
  `cmd/spinloop/main_test.go`, `cmd/spinloop/complete_test.go`, and any harness tests that
  pass a family.
- Docs: `docs/spinloop-file.md`, `docs/commands/list.md`, `docs/commands/export.md`,
  `docs/commands/add.md`/`remove.md` where `FAMILY`/`-f` appear, `AGENTS.md`.
- Backward compatibility: existing Spinloop files or scripts using `FAMILY`/`--model-family`
  will fail to parse and must switch to `MODEL`/`ALIAS`. No shipped example uses `FAMILY`.
