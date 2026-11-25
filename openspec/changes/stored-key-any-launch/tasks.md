# Tasks

## 1. Launch key lookup

- [x] 1.1 Pass the harness to `storedKeyResolver` and, with no applied Spinloop, choose providers from `h.State()`: the default model's provider, or every configured provider when there is no default
- [x] 1.2 Wrap the resolver for each chosen catalogue provider with `apiKeyFile`, at the base URL from the state or `discovery.ResolveBaseURL`

## 2. Tests

- [x] 2.1 Bare launch: the default model's local provider gets the stored key
- [x] 2.2 Bare launch: a provider configured at a remote base URL gets no key
- [x] 2.3 Bare launch: a harness with no default model gets each configured local provider's key
- [x] 2.4 Bare launch: a different default provider, an unknown provider id, or a `State()` error leaves the key unset
- [x] 2.5 Applied Spinloop behaviour is unchanged

## 3. Docs

- [x] 3.1 Update the Unsloth key section of `docs/commands/serve.md` and the troubleshooting key entry

## 4. opencode file reference

- [ ] 4.1 Add `(*Provider).StoredKeyPath()` and have `StoredKey` read through it
- [ ] 4.2 In `BuildProviderBlock`, write `{file:<path>}` when the variable is unset, the endpoint is local and a stored key exists
- [ ] 4.3 Tests: opencode gets the file reference; a set variable keeps the env reference; a remote endpoint, an empty or missing file keep the current behaviour; Pi is unchanged
- [ ] 4.4 Docs: the Unsloth key section and the troubleshooting entry (one service restart after switching)
