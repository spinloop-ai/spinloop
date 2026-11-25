# Tasks

## 1. Launch key lookup

- [ ] 1.1 Pass the harness to `storedKeyResolver` and, with no applied Spinloop, choose providers from `h.State()`: the default model's provider, or every configured provider when there is no default
- [ ] 1.2 Wrap the resolver for each chosen catalogue provider with `apiKeyFile`, at the base URL from the state or `discovery.ResolveBaseURL`

## 2. Tests

- [ ] 2.1 Bare launch: the default model's local provider gets the stored key
- [ ] 2.2 Bare launch: a provider configured at a remote base URL gets no key
- [ ] 2.3 Bare launch: a harness with no default model gets each configured local provider's key
- [ ] 2.4 Bare launch: a different default provider, an unknown provider id, or a `State()` error leaves the key unset
- [ ] 2.5 Applied Spinloop behaviour is unchanged

## 3. Docs

- [ ] 3.1 Update the Unsloth key section of `docs/commands/serve.md` and the troubleshooting key entry
