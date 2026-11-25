# Tasks

## 1. Catalogue

- [x] 1.1 Add `APIKeyFile` (`apiKeyFile`) to `catalog.Provider`
- [x] 1.2 Add `(*Provider).StoredKey()`: expand env refs, glob, read the newest readable match, trim whitespace
- [x] 1.3 Add `(*Provider).KeyResolver(resolve, baseURL)`: answer `APIKeyEnv` from `resolve`, else the stored key when the endpoint is local
- [x] 1.4 Wrap `resolve` with `KeyResolver` in `BuildProviderBlock` and `BuildPiProvider` once the base URL is known
- [x] 1.5 Add `apiKeyFile: $HOME/.unsloth/studio/auth/.cli_api_key_cli_*` to the `unsloth` provider

## 2. Launch and discovery

- [x] 2.1 In `launchAgent`, wrap the resolver with the worn selection's provider `KeyResolver` (not for cloud or fleet launches), using the base URL the apply used
- [x] 2.2 Wrap the resolver in `discovery.Models` so completion can list a local Studio's models

## 3. Tests

- [x] 3.1 `StoredKey`: no pattern, no match, empty file, whitespace trimmed, newest of several, `$HOME` expansion
- [x] 3.2 `KeyResolver`: variable wins, file fills a gap for local, file ignored for remote, other names passed through
- [x] 3.3 opencode block and Pi entry for `unsloth` with a stored key: env reference, not the secret
- [x] 3.4 Launch environment carries `UNSLOTH_API_KEY` from the stored key, with `.env` and exported values taking precedence
- [x] 3.5 The embedded `unsloth` provider declares the Studio key file

## 4. Docs

- [ ] 4.1 Update the Unsloth Studio section of `docs/commands/serve.md` and the key entry in `docs/env-vars.md`
- [ ] 4.2 Document `apiKeyFile` where the catalogue fields are described, and the opencode service caveat in troubleshooting
