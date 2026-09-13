## Context

See proposal.md for the motivation. The implementation-relevant current state:

- `remote/lambda/env/index.ts` (the `EnvFn` in `remote/lib/llm-stack.ts`, part
  of the account-level control-plane stack `spinloop remote bootstrap` stands
  up and updates) reads the Elastic IP and the Secrets Manager key, and
  replies with only `base_url`/`api_key`. It does not touch the deploy-config.
- The deploy-config lives in SSM at the path `deployConfigParam(env)`
  (`remote/lambda/shared/environments.ts`), written by the `deploy` Lambda and
  already read by `start` (`readDeployConfig`, `remote/lambda/start/index.ts`)
  and by `stats`, both of which relay `runner`, `modelId`, `servedName` and
  `contextSize` in their own replies. `parseDeployConfig` already tolerates a
  missing/invalid parameter value on the read side used elsewhere (`start`
  falls back to reporting `deployed: false`).
- `internal/remote.Response` (`internal/remote/remote.go`) already declares
  `Deployed`, `Runner`, `ModelID`, `ServedName`, `ContextSize` — added for the
  `start`/`stats` replies. `Env()` unmarshals into the same struct, so once the
  Lambda sends the fields, the Go client already parses them; no struct change
  needed there.
- `cmd/spinloop/main.go`'s `fetchRemoteEnv` calls `remote.Env` and returns
  the raw `*remote.Response` up to `applyRoutedSpinloop`, which today only
  reads `BaseURL`/`APIKey` off it. `applyRoutedSpinloop` (and the
  `applyBeforeLaunch` that calls it) only run when a Spinloop was read — see
  `harnessCmd`'s `RunE` in `cmd/spinloop/commands.go`, which calls
  `applyBeforeLaunch` only `if spinloopPath.set`. With no Spinloop, `--env`
  is parsed into `route.envName` and then never read.
- `cmd/spinloop/remote.go`'s `runnerFor(provider string)` maps a Spinloop's
  `PROVIDER` to a deploy `Runner` with an identity mapping for `llamacpp` and
  `vllm` (the only deployable engines) — reversible without ambiguity.
  `deriveDeployTarget` sets `dc.ServedModelName = sel.Alias`, falling back to
  the model id when the Spinloop states no `ALIAS`.
- `applySelection` (`cmd/spinloop/main.go`) is the single place that writes a
  harness config from a `spinloop.Selection`; it requires `sel.Model != "" ||
  sel.Alias != ""`, looks up `cat.Providers[sel.Provider]` for the engine
  definition, then (when `envName != ""`) relabels the provider key to the
  environment name and takes the base URL from the environment's registered
  `remote.json` when the selection states none. This is the exact path that
  must run for an auto-configured launch too, unchanged, so the resulting
  config is indistinguishable from one a Spinloop produced.

## Goals / Non-Goals

**Goals:**

- `spinloop harness --env <name>` with no Spinloop applied configures and
  launches the harness entirely from what is live at the environment: no file
  has to travel between the machine that deployed and the machine that
  launches, beyond the registry entry (`remote.json`) needed to reach the
  control plane at all.
- The auto-configured path and the Spinloop-driven path converge on the same
  `applySelection` call, so there is exactly one place that writes a harness
  config from a provider selection, and one set of tests for it.
- A clear, fatal error distinguishes "nothing to auto-configure from" from
  every other `--env` failure already specified (unregistered environment,
  AWS credentials, no key available) — it must not be mistaken for one of
  those.

**Non-Goals:**

- No change to what `remote deploy` stores or how `--api-key-env`/rotation
  work; this only adds a *read* of state that already exists.
- No attempt to keep an old, un-bootstrapped `env` Lambda working with the new
  behaviour — the fix is `spinloop remote bootstrap`, not a compatibility
  shim in the CLI.
- No local caching of a fetched deploy-config; every bare `--env` launch is a
  fresh live query, which is the point (a redeploy is picked up automatically,
  never goes stale).
- No change to the `remote-environments` registry format (`remote.json`'s
  schema is untouched) — the deploy-config lives only in SSM and only travels
  over the wire, never to disk on the caller's machine.

## Decisions

**D1: Enrich the `env` Lambda, not `remote.json`.** The alternative — writing
runner/model/context into the registered environment's `remote.json` at
deploy time — was the shape the user first proposed (copy `remote.json`
between machines). Rejected: `remote.json` already means "how to reach the
control plane," and baking model facts into it creates a second source of
truth that goes stale the moment someone redeploys a different model without
every machine re-copying the file. Reading `env`'s live reply instead means
the fact is asked for fresh on every launch and can never disagree with what
is actually running.

**D2: Reuse `Response`, add no new wire shape.** `start`/`stats` already
return `runner`/`modelId`/`servedName`/`contextSize`/`deployed` in this exact
JSON shape, and `internal/remote.Response` already parses them. Giving `env`
a different shape for the same facts would mean two parsers for one concept.
The `env` Lambda's TypeScript reply gains the same field names; the Go client
needs no change to `Response`, only a new caller that reads it from an `env`
result instead of a `start`/`stats` one.

**D3: Best-effort read, never fail the whole `env` call.** `parseDeployConfig`
already has to tolerate absence (an environment can be registered with
nothing deployed) on the `start` path, which reports `deployed: false` rather
than erroring. `env` follows the same rule: a missing or unparsable
deploy-config omits the fields; `base_url`/`api_key` are still useful on
their own (the existing credential-injection behaviour must not regress).

**D4: The CLI decides whether "no deploy-config" is fatal, not the Lambda.**
Whether an absent deploy-config is fine (a Spinloop is supplying the model
info) or fatal (nothing else can) depends on whether a Spinloop was applied —
information the Lambda doesn't have and shouldn't need. So `env` always
degrades gracefully per D3, and `cmd/spinloop` is where the two cases
(Spinloop present vs. absent) diverge.

**D5: One synthesis point, immediately before the existing `applySelection`
call.** `applyRoutedSpinloop` already has the fetched `*remote.Response` in
scope (as `remoteResp`) at the point it calls `applySelection`. When no
Spinloop was applied, a small step ahead of that call builds a
`spinloop.Selection{Provider: <runner-mapped>, Alias: resp.ServedName,
Context: resp.ContextSize}` from `remoteResp` and hands it to the *same*
`applySelection`, rather than adding a parallel write path. Alternatives
considered: a separate `applyFromEnvironment` function duplicating
`applySelection`'s provider-lookup and relabelling logic (rejected — the
whole point of D5 is that a config built this way must be indistinguishable
from one a Spinloop produced, which a duplicate risks drifting from over
time).

**D6: `harnessCmd`'s gate moves from "a Spinloop is set" to "a Spinloop is
set, or `--env` is given."** Today `RunE` only calls `applyBeforeLaunch` `if
spinloopPath.set`; a bare `--env` with nothing else currently launches the
harness completely unconfigured, silently ignoring the flag. That gate widens
to also enter the apply path when `route.envName != ""`, and
`applyRoutedSpinloop` picks which selection to build depending on whether a
Spinloop was actually read. This is a widening of when the apply path runs,
not a new flag or a new command surface — the flag already means the same
thing conceptually, it just used to require a Spinloop to be honoured at all.

**D7: Runner → catalogue-provider mapping is shared, not re-derived.**
`runnerFor` already encodes the one-directional (provider → runner) mapping
at deploy time; a fetched `Runner` needs the reverse. Since the mapping is
currently an identity function for both deployable engines (`llamacpp`,
`vllm`), the reverse direction is extracted as a small shared helper next to
`runnerFor` rather than hand-rolled again at the call site — so the day a
non-identity runner is added, one place has to change, not two.

## Risks / Trade-offs

- **[Risk]** Existing environments' `env` Lambda predates this change and
  will keep replying without the new fields until the account re-runs
  `spinloop remote bootstrap`. → Mitigation: this is D4's fatal-error path
  already, and the error names `spinloop remote bootstrap` as the fix (see
  the `harness-remote-env` delta's "env Lambda predating this behaviour"
  scenario) — the same remediation an operator already needs for any other
  control-plane upgrade.
- **[Risk]** A live fetch on every bare `--env` launch adds one network round
  trip (already happens today for credential injection whenever `--env` is
  given with a Spinloop, so this is not a new cost — it is the same fetch,
  now also read for two more fields, on a path that previously skipped it
  entirely).
- **[Risk]** Someone deploys a hosted (non-self-hosted) provider's Spinloop
  in the future in a way that produces a `Runner` value with no catalogue
  provider counterpart. → Mitigation: `runnerFor` already rejects any
  `PROVIDER` that isn't `llamacpp`/`vllm` at deploy time, so no other runner
  value can ever reach SSM; the reverse mapping only ever has to handle the
  same two names.

## Migration Plan

1. Add the deploy-config read to `remote/lambda/env/index.ts`, reusing the
   `readDeployConfig`/`deployConfigParam` helpers `start` already imports, and
   grant the env Lambda's role read-only `ssm:GetParameter` on the
   deploy-config parameter in `remote/lib/llm-stack.ts` — easy to miss, since
   the read still "succeeds" from the CLI's point of view: an `AccessDenied`
   is caught by the same best-effort handling that covers "nothing deployed
   yet," so the two look identical without the CDK-level test added for it.
2. Extend `internal/remote` only if a gap is found in what `Response` already
   captures for `start`/`stats` (expected: none — see D2).
3. Add the runner→provider reverse mapping next to `runnerFor` in
   `cmd/spinloop/remote.go`.
4. Wire the synthesis step into `applyRoutedSpinloop` per D5, and widen
   `harnessCmd`'s gate per D6.
5. Update `docs/commands/harness.md` and `docs/commands/remote.md`.
6. No code rollback concern beyond a normal revert: the Lambda change is
   additive (new optional fields in a JSON reply) and the CLI change only
   takes a new path when `--env` is given with no Spinloop, which today does
   nothing — there is no existing behaviour to regress for that input.
   Existing accounts opt in by running `spinloop remote bootstrap`, at their
   own pace; a CLI upgrade alone changes nothing until they do.
