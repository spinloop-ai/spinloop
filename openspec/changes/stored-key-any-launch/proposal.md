# Proposal

## Why

When a launch names a cloud node, the provider being launched determines where
its key comes from: spinloop fetches the key and gives it to the harness. The
local path does not follow the same rule. v1.51.0 reads a provider's stored key
(`apiKeyFile`) only when the launch applies a Spinloop. `spinloop code` run
from a directory with no Spinloop launches the harness with the config already
on disk. When that config's model is served by Unsloth Studio, the agent gets
no `UNSLOTH_API_KEY`, and every request returns 401 "Not authenticated".

## What Changes

- A launch finds the provider the harness will use, and if that provider
  needs a key that is set nowhere, it fetches the key from the provider's own
  source and gives it to the harness. This now applies whether or not the
  launch applies a Spinloop.
- The provider is the applied Spinloop's when there is one. Otherwise it is
  the provider of the harness's default model. For a harness with no default
  model, it is every provider its config holds.
- The endpoint checked for being local is the one the apply writes for an
  applied Spinloop, otherwise the base URL in the harness's config, otherwise
  the catalogue's endpoint.
- The cloud path is unchanged.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `provider-catalog`: the "Embedded provider definitions" requirement states
  which provider a launch looks up a stored key for, and the endpoint it
  checks.

## Impact

- `cmd/spinloop/commands.go`: the launch's key lookup.
- Docs: the Unsloth key section in `docs/commands/serve.md` and the
  troubleshooting key entry.
- PR 286 (cloud nodes on the control plane) is expected to land first and
  touches `commands.go`, but not the launch's key lookup. This change stays
  out of the cloud code it refactors.
