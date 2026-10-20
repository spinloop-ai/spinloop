# Proposal

## Why

Arbitrary engine options today only reach the engine through an external
`preset.ini` named by `PRESET`. That forces a second file (and its own
resolution and fetch rules) to mix and match a handful of options that the
author would keep in the Spinloop anyway. A repeatable `PARAM` instruction
carries engine options inline — the issue's Qwen3.8-27B Spinloop, from
sampling parameters to flash attention to speculative decoding, in one file.

## What Changes

- New repeatable `PARAM` instruction in the Spinloop file: `PARAM key = value`,
  in the engine's own flag vocabulary, read the way a preset key is read (the
  engine's dialect does the aliasing and boolean handling).
- `spinloop serve` takes the Spinloop's `PARAM`s as an override layer: they
  beat a preset's values and lose to the Spinloop's own instructions
  (`MODEL`, `ALIAS`, `CONTEXT`, `PARALLEL`, `BASEURL`). A PARAM that names the
  engine's model source (llama.cpp's `hf`/`model`, MTPLX's `model`) satisfies
  serve's model requirement in place of `MODEL` — so a Spinloop with no
  `MODEL` but a `PARAM hf = …` serves, exactly as a preset-driven one does.
  vLLM takes its model positionally, so it still needs `MODEL`.
- `spinloop cloud deploy` (and the fleet-node wake, which derives its config
  the same way) reads `PARAM`s exactly as it reads a preset: the model,
  context-size and parallelism fallbacks consult the PARAM layer, and the
  remaining PARAMs become the engine's serveArgs minus the settings the
  destination owns.
- `apply`/`unapply` ignore `PARAM` (as they already ignore `PRESET`);
  `export` does not emit it.
- Two new examples using `PARAM` instead of a preset file:
  `examples/llamacpp/qwen3.8-27b-mtp/` (the issue's Q5_K_XL + MTP
  speculative-decoding configuration) and `examples/vllm/qwen3.8-27b/` (the
  first vLLM example).
- Docs: the Spinloop file page gains `PARAM` in its keyword table and rules,
  and `serve`'s help and docs name it.

## Capabilities

### New Capabilities

(None — the behaviour lands in three existing capabilities.)

### Modified Capabilities

- `spinloop-files`: a new repeatable `PARAM` instruction in the file grammar
  (syntax, repetition, parse errors); `apply`/`unapply` ignore it; the unknown
  keyword list and `export`'s canonical output change accordingly.
- `local-serving`: `PARAM` is rendered into the engine command in the engine's
  dialect as a layer between the preset and the Spinloop's own instructions;
  it may supply the model for engines that take one as a flag.
- `remote-endpoint`: the deploy derivation reads the PARAM layer for the
  model, context-size and parallelism fallbacks and folds the remaining PARAMs
  into the serveArgs, with the same destination-owned dropping as a preset.

## Impact

- `internal/spinloop` — parser, `Selection`, `Format`; tests.
- `cmd/spinloop/serve.go` — engine param layers, model-requirement check; tests.
- `cmd/spinloop/cloud.go` — `deployConfig` fallbacks and serveArgs; tests.
- `docs/spinloop-file.md`, `docs/commands/serve.md`, serve help text.
- `examples/llamacpp/qwen3.8-27b-mtp/`, `examples/vllm/qwen3.8-27b/`.
- No API, dependency, or harness-config change; existing Spinloops parse and
  behave exactly as before.
