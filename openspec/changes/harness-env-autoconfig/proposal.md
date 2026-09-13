## Why

`spinloop harness --env <name>` only injects a remote endpoint's credentials
into the launched agent's environment when a Spinloop is also applied — the
provider kind and model key still have to come from a local `Spinloop` file,
even though the environment already knows what it is serving. An operator on
a second machine who only has (or only wants) the registered environment has
no way to launch against it: they either write a throwaway Spinloop that
duplicates what `remote deploy` already recorded, or go without. The control
plane already stores exactly this information — the runner, the served model
name, and the context size are written to SSM at deploy time and already
relayed by the `start` and `stats` Lambdas — the `env` Lambda `spinloop
harness --env` calls just does not read it yet.

## What Changes

- The `env` Lambda additionally reads the environment's deploy-config from SSM
  (best-effort: an environment can be registered with nothing deployed to it,
  or predate this change) and returns `deployed`, `runner`, `modelId`,
  `servedName`, and `contextSize` alongside the existing `base_url` and
  `api_key`, mirroring the fields `start`/`stats` already relay.
- `spinloop harness --env <name>` SHALL configure and launch the harness from
  that response when no Spinloop is applied: the runner becomes the catalogue
  provider (the same identity mapping `remote deploy` already uses in
  reverse), the served name becomes the model key, and the context size sets
  the window — going through the same `applySelection` path a Spinloop-driven
  apply already uses, so the written config is indistinguishable from one a
  Spinloop produced.
- A bare `spinloop harness --env <name>` against an environment with nothing
  deployed, or whose `env` Lambda predates this change (the response simply
  omits `deployed`/`runner`/etc.), SHALL fail with an actionable error rather
  than launch an unconfigured or stale harness — naming the environment and
  saying to deploy it, or to `spinloop remote bootstrap` to pick up the
  updated Lambda.
- Applying a Spinloop alongside `--env` is unaffected: an explicit `PROVIDER`,
  `ALIAS`, `MODEL` or `CONTEXT` in the Spinloop continues to win, exactly as a
  hand-written `BASEURL` already wins over the environment's registered
  address.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `remote-env`: the `env` Lambda additionally returns the environment's
  deploy-config (`deployed`, `runner`, `modelId`, `servedName`,
  `contextSize`) when one is registered and parses; the "no boot" requirement
  is reworded to cover the added SSM read, which is still boot-free.
- `harness-remote-env`: `spinloop harness --env <name>` with no Spinloop
  applied SHALL synthesise a provider selection from the environment's
  deploy-config instead of doing nothing with the flag; an environment with
  nothing deployed, or an `env` Lambda that predates this change, SHALL fail
  the launch naming the cause and the fix, rather than launching unconfigured.

## Impact

- `remote/lambda/env/index.ts`: reads `deployConfigParam(env)` via the shared
  `readDeployConfig` helper (already used by `start`/`stats`) and adds the
  fields to its JSON reply; a missing or unparsable deploy-config degrades to
  omitting them rather than failing the whole response, since `base_url` and
  `api_key` remain valid without it.
- `internal/remote/remote.go`: `Response` already carries `Deployed`,
  `Runner`, `ModelID`, `ServedName`, `ContextSize` — no struct change, just a
  new caller reading them from an `env` reply instead of only a `start`/
  `stats` one.
- `cmd/spinloop/commands.go` (`harnessCmd`) and `cmd/spinloop/main.go`
  (`applyBeforeLaunch`/`applyRoutedSpinloop`/`fetchRemoteEnv`): the apply path
  currently only runs when a Spinloop is worn; it gains a route that
  synthesises a `spinloop.Selection` from a fetched environment's deploy-config
  when `--env` is given and no Spinloop is applied.
- `cmd/spinloop/remote.go`: the `runnerFor`/catalogue-provider identity
  mapping used at deploy time is reused (or a shared helper extracted) to map
  a fetched `Runner` back to a catalogue provider name.
- Docs: `docs/commands/harness.md`, `docs/commands/remote.md`.
- Existing deployed accounts need `spinloop remote bootstrap` re-run (a CDK
  stack update) before their `env` Lambda returns the new fields; until then,
  `spinloop harness --env <name>` with no Spinloop continues to fail with the
  same actionable error as an undeployed environment.
