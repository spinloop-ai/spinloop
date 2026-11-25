# Proposal

## Why

Unsloth Studio requires an API key on `/v1`. Studio generates that key itself,
and nothing lets the user choose it. A Spinloop naming `PROVIDER unsloth`
therefore launches a harness that gets 401 "Not authenticated" until the user
copies the key out of Studio's startup banner into `UNSLOTH_API_KEY` or a
`.env`. Studio stores the key in a file under `~/.unsloth/studio/auth/` and
reuses it on every run, so spinloop can read that file when the variable is
not set.

## What Changes

- The provider catalogue gains an optional `apiKeyFile` field: a path pattern
  (glob, with `$HOME`-style environment references) to a file holding the
  provider's key, used when the key variable is set nowhere.
- That file is read only when the provider's endpoint is local, so a key
  stored for a local server is never sent to a remote host.
- When several files match, the most recently modified one is used. Surrounding
  whitespace is trimmed.
- The key variable keeps precedence: an `ENV` instruction, then the process
  environment, then the `.env` beside the Spinloop, then the key file.
- `spinloop harness apply` and `spinloop code` treat a key found in the file as
  set. For opencode, the provider block references `UNSLOTH_API_KEY` instead of
  leaving out the key, and Pi gets `$UNSLOTH_API_KEY` instead of the keyless
  placeholder. When a harness is launched, `UNSLOTH_API_KEY` is put in its
  environment.
- The `unsloth` provider declares
  `apiKeyFile: $HOME/.unsloth/studio/auth/.cli_api_key_cli_*`, the file
  `unsloth run` writes for its default key name, `cli`.
- No secret is written to disk by spinloop. Harness configs still hold only a
  reference to the variable.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `provider-catalog`: the embedded provider definitions gain the optional
  `apiKeyFile` field and its resolution rules, and the Unsloth Studio provider
  reads Studio's stored key through it.

## Impact

- `internal/catalog`: new `APIKeyFile` field, a function that reads the stored
  key, and its use in the opencode and Pi block builders.
- `cmd/spinloop`: the launch environment and lucinate's launch key use the
  stored key for the worn selection's provider.
- `internal/catalog/providers.yaml`: `apiKeyFile` on `unsloth`.
- Docs: Unsloth sections in `docs/commands/serve.md`, `docs/env-vars.md`, and
  the catalogue field reference.
