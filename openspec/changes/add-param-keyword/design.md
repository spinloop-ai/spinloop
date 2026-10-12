# Design: PARAM keyword

## Context

The Spinloop grammar (`internal/spinloop`, a pure-grammar leaf — stdlib only, no
I/O, never imports `internal/preset`) parses `KEYWORD value` lines where every
instruction except `ENV` must appear once and take exactly one value. Engine
options beyond the typed instructions (`MODEL`, `ALIAS`, `CONTEXT`, `PARALLEL`,
`BASEURL`) today reach the engine only through an external preset `.ini`
(`PRESET`), read in the engine's dialect: `preset.Dialect.Flags(layers...)`
merges ordered layers with later-wins by canonical flag name.

Two consumers build commands/configs from a Spinloop, and both already treat
the preset as a layer below the Spinloop's own instructions:

- `cmd/spinloop/serve.go` — `buildServeArgv`: preset path via
  `Preset.CommandIn(dialect, binary, sub, sec, overrides...)` (variadic
  override layers, already in place); preset-less path via
  `assembleEngineArgv` rendering `engine.dialect.Flags(params)` for one layer
  (the engine's `params(sel)` output).
- `cmd/spinloop/cloud.go` — `deployConfig(sel, path, target)`: reads the
  preset's model/context/parallel keys as fallbacks behind the Spinloop's own
  values (`presetValue(key, global, section)`), then layers
  `[dropOwned(owns, global), dropOwned(owns, section)]` (plus the bind for
  node wakes) into `ServeArgs`, and reads companion weights from the same
  layers. `deployConfigForNode` reuses it with a different `owns` set.

The issue fixes one precedence rule: where a param is set both in the preset
and as a `PARAM`, the Spinloop file wins.

## Goals / Non-Goals

Goals (behaviour in the spec deltas): `PARAM` parses like `ENV` (repeatable),
renders in the engine's dialect, sits between the preset and the Spinloop's
typed instructions, may supply the model for engines that take it as a flag,
and flows to cloud deploy and fleet node wakes with the same owned-flag
dropping a preset gets.

Non-goals: the `hf` command (it emits MODEL-based Spinloops, no PARAM);
`harness export` (a harness config cannot hold engine flags, so there is
nothing to emit); shell completion (no Spinloop-keyword completion exists);
the daemon's push path (it reads a `DeployConfig`, where PARAM is already
baked into `ServeArgs` by `deployConfig`).

## Decisions

### 1. PARAM is stored as its own type in `internal/spinloop`

`Selection` gains `Params []Param` with a small `Param{Key, Value string}`
type defined in the grammar leaf. Alternative — reusing `preset.Param` —
rejected: it would make the pure-grammar leaf import `internal/preset`,
breaking the stated invariant that the leaf never imports outside itself.
`cmd/spinloop` converts `Selection.Params` to `[]preset.Param` at the two
consumers (serve, cloud), which already import both.

### 2. Precedence: typed instructions > PARAM > preset

Layer order is `[preset global, preset section, PARAM, Spinloop instructions]`
for serve, and `[dropOwned(global), dropOwned(section), dropOwned(PARAM)]`
(plus bind) for deploy. The issue mandates PARAM-over-preset;
PARAM-under-instructions is the design call, because the typed instructions
are the canonical interface and own side effects — the llamacpp
`ctx-size * parallel` scaling runs off `sel.Context`/`sel.Parallel`, not off
flags, so a `PARAM ctx-size` cannot sneak a second context into the command.
Alternative — PARAM beats everything — rejected: it would let a raw
pass-through silently defeat `CONTEXT`/`PARALLEL`, breaking the scaling
contract the `local-serving` spec pins down.

### 3. Reuse the dialect's merge rather than invent one

`Dialect.Flags`'s layer merge (later-wins by canonical key, first-appearance
order) is extracted to `Dialect.Merge(layers ...[]Param) []Param`; `Flags`
becomes render-of-`Merge`. This gives:

- preset path: `CommandIn` already takes variadic override layers, so PARAM is
  passed as one more: `CommandIn(d, bin, sub, sec, selParams, params)` — no
  signature change;
- preset-less path: `assembleEngineArgv` receives
  `engine.dialect.Merge(selParams, params)` as its single params argument —
  no signature change;
- collision semantics for a PARAM against a preset (`ngl` vs `n-gpu-layers`)
  are identical to preset-vs-preset, for free.

Duplicate PARAM keys are kept in file order in the selection; the dialect
merge resolves them last-wins, the same way a preset section's repeated keys
behave.

### 4. Model sourcing is per-engine, defined once

`cmd/spinloop` gains `modelSourceFlags(runner) []string` — the canonical flag
names that name the engine's model: llamacpp `hf-repo`, `model`, `hf-file`;
mtplx `model`; vllm none (the model is positional, so `PARAM` cannot stand in
for `MODEL`). One definition serves both sides: serve's `needsModel` check
(`MODEL` or a PARAM naming the model, for engines with model flags) and
cloud's PARAM model fallback (`sel.Model` → preset's model key → PARAM's
model keys). A PARAM whose model value is a local path deploys exactly like a
`MODEL` local path does today: the cloud refuses it naming a repo.

### 5. Cloud parity is layer addition, not a new path

`deployConfig` converts `sel.Params` once to `[]preset.Param` and then:
extends each of the model, context and parallel fallbacks with
`presetValue(key, paramLayer)` (the existing variadic helper scans the new
layer last), appends `dropOwned(owns, paramLayer)` as the final ServeArgs
layer before the bind, and passes the PARAM layer to `companionsFrom`.
Because `owns` is the seam, `PARAM host = 0.0.0.0` drops for a cloud deploy
(`isCloudOwned`) and survives for a node wake (`isNodeOwned`) — the same
asymmetry a preset's bind has today. The two "nothing to deploy"/"no context
size" errors gain a naming of PARAM as an alternative.

## Risks / Trade-offs

- [PARAM sets `ctx-size`/`parallel` for llamacpp] → the layering keeps the
  Spinloop's typed values on top, and the scaling contract runs off
  `sel.Context`/`sel.Parallel`; a PARAM's own `ctx-size` is treated as a
  preset's is — passed through unscaled.
- [The parser's "exactly one value" rule] → `PARAM` is special-cased before
  the field-count check, as `ENV` is after it; its value is the rest of the
  line split at the first `=`, so values may contain spaces and `=` signs
  (e.g. a JSON chat template), while the key must stay one token.
- [vllm/omlx/mtplx dialects have no boolean table] → a boolean flag for those
  engines takes an empty value to render bare (`PARAM enable-chunked-prefill =`);
  a non-empty value renders as `--flag value`. This is exactly the constraint
  a preset for those engines has today, so PARAM is parity, not a regression;
  the vllm example uses value-taking flags only.
- [A PARAM key the engine does not know] → passed through verbatim in the
  engine's dialect, the same way an unknown preset key is: the engine's own
  flag validation is the backstop, unchanged.

## Migration Plan

Additive: existing Spinloops parse and behave identically (no PARAM → the new
layers are empty and `Merge` of one layer equals today's `Flags` of it).
Rollback is reverting the change; nothing on disk or in a deployed config
gains a new shape — a deployed `DeployConfig` with PARAM-derived `ServeArgs`
is already just engine flags.

## Open Questions

None — the precedence, model sourcing, and cloud parity decisions are settled
here and in the spec deltas.
