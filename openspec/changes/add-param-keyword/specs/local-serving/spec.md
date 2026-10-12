## MODIFIED Requirements

### Requirement: Preset-less serving

Without a `PRESET`, the Spinloop SHALL supply the command directly, and the
model SHALL be named by a `MODEL` or, for an engine that takes its model as a
flag, by a `PARAM` naming it (see PARAM instructions): a `MODEL` value that
looks like a local file (ending `.gguf`, or starting with `/`, `./`, `../`, or
`~`) becomes the model path, anything else is treated as a Hugging Face repo
reference. `ALIAS` sets the served model's reported name, `CONTEXT` (parsed
with the standard lenient size format) sets the context size, and `BASEURL`'s
host and port set the server's bind address.

#### Scenario: Hugging Face repo

- **WHEN** the Spinloop has `MODEL unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL`,
  `ALIAS qwen3.6`, and `CONTEXT 32768`
- **THEN** the command carries the repo reference, the alias, and a context
  size of 32768

#### Scenario: Local file

- **WHEN** the Spinloop has `MODEL ./models/qwen.gguf`
- **THEN** the command loads that file as a model path rather than a repo

#### Scenario: No model to serve

- **WHEN** the Spinloop has neither `PRESET` nor `MODEL`, and no `PARAM`
  naming the model for an engine that takes its model as a flag
- **THEN** the command fails explaining serve needs a preset, a model, or a
  PARAM naming the model

### Requirement: Flag rendering

Preset keys and PARAM keys SHALL render as the engine's flags. Where an engine
defines short aliases, they SHALL be canonicalised to their long form so the
same flag written different ways collapses to one when layers merge, later
layers overriding earlier ones in place; an engine that defines none SHALL pass
its keys through unchanged. Flags the engine accepts bare SHALL render bare
when truthy and be dropped when falsy (`0`, `false`, `off`, `no`). Printed
commands SHALL quote only the tokens that need it, and SHALL pass values to the
server verbatim.

#### Scenario: Layer override by canonical name

- **WHEN** the global section sets `c = 4096` and the model section sets
  `ctx-size = 8192` for llama.cpp
- **THEN** the command carries a single context-size flag with value 8192

#### Scenario: Boolean flags

- **WHEN** a llama.cpp preset sets `jinja = 1` and `mmap = 0`
- **THEN** the command includes a bare `--jinja` and no mmap flag

#### Scenario: An engine with no short aliases

- **WHEN** an oMLX preset sets `model-dir` and `memory-guard`
- **THEN** both render as their long-form flags with their values unchanged

### Requirement: Parallelism

`CONTEXT` SHALL always mean the context window a single request gets, whatever
engine serves it. A Spinloop's `PARALLEL` SHALL set the number of concurrent
request slots, translated per engine so that `CONTEXT`'s meaning holds:

- For `llamacpp`, `PARALLEL n` SHALL be rendered as `--parallel n`. Because
  llama.cpp treats `--ctx-size` as a total budget divided across its parallel
  slots, when the Spinloop **also** states `CONTEXT`, the rendered `--ctx-size`
  SHALL be `context_tokens * n` rather than `context_tokens`, so each slot
  still gets the context the Spinloop asked for. `CONTEXT` set with no
  `PARALLEL` SHALL render `--ctx-size` as `context_tokens`, unscaled, exactly
  as before this capability existed.
- For `vllm`, `PARALLEL n` SHALL be rendered as `--max-num-seqs n`.
  `--max-model-len` (from `CONTEXT`) SHALL NOT be scaled by `PARALLEL`: vLLM's
  concurrency is bounded independently of a single request's context length.
- For `omlx`, `PARALLEL n` SHALL be rendered as `--max-concurrent-requests n`.
  oMLX has no context flag to scale either way.
- For `mtplx`, `PARALLEL n` SHALL be rendered as `--max-active-requests n`,
  MTPLX's cap on admitted concurrent requests. `--context-window` (from
  `CONTEXT`) SHALL NOT be scaled by `PARALLEL`. MTPLX's scheduling mode — how
  admitted requests execute, serially or in parallel — SHALL remain a preset
  concern: a Spinloop's `PARALLEL` SHALL NOT select it.

A Spinloop stating no `PARALLEL` SHALL produce a command identical to one from
before this capability existed, for all four engines. A `PARALLEL` value
SHALL be validated as a positive integer at the point it is used (`serve`, a
daemon-pushed config, or `cloud deploy`); a value that is not SHALL fail
naming the invalid value rather than being passed to the engine.

When `CONTEXT` is supplied by a `PRESET` section's own `ctx-size` or a `PARAM`'s
own `ctx-size` rather than by the Spinloop, `PARALLEL` SHALL NOT rescale it —
only a Spinloop-stated `CONTEXT` participates in the `llamacpp` multiply. A
`PARALLEL` value SHALL still override a preset's own or a `PARAM`'s own
`np`/`parallel` (llama.cpp), `max-num-seqs` (vLLM),
`max-concurrent-requests` (oMLX), or `max-active-requests` (MTPLX) value by the
same override-by-canonical-name rule `CONTEXT` already uses against a preset's
`ctx-size`.

#### Scenario: llama.cpp context is scaled by parallel slots

- **WHEN** a Spinloop states `PROVIDER llamacpp`, `CONTEXT 128k`, and
  `PARALLEL 2`
- **THEN** the printed command includes `--ctx-size 256000` and `--parallel 2`

#### Scenario: llama.cpp parallel with no context set

- **WHEN** a Spinloop states `PROVIDER llamacpp` and `PARALLEL 4` with no
  `CONTEXT`
- **THEN** the printed command includes `--parallel 4` and no `--ctx-size`
  flag is added

#### Scenario: llama.cpp context with no parallel set is unscaled

- **WHEN** a Spinloop states `PROVIDER llamacpp` and `CONTEXT 128k` with no
  `PARALLEL`
- **THEN** the printed command includes `--ctx-size 128000`, exactly as it
  would have before `PARALLEL` existed

#### Scenario: A preset's own context is not rescaled

- **WHEN** a Spinloop states `PROVIDER llamacpp`, `PARALLEL 2`, and a `PRESET`
  whose section sets `ctx-size` but the Spinloop itself states no `CONTEXT`
- **THEN** the printed command includes `--parallel 2` and the preset's
  `ctx-size` value passes through unscaled

#### Scenario: vLLM concurrency does not touch context

- **WHEN** a Spinloop states `PROVIDER vllm`, `CONTEXT 128k`, and `PARALLEL 4`
- **THEN** the printed command includes `--max-model-len 128000` and
  `--max-num-seqs 4`, with neither value derived from the other

#### Scenario: oMLX concurrency

- **WHEN** a Spinloop states `PROVIDER omlx` and `PARALLEL 8`
- **THEN** the printed command includes `--max-concurrent-requests 8` and no
  context flag, exactly as with no `PARALLEL` stated

#### Scenario: MTPLX concurrency does not touch context

- **WHEN** a Spinloop states `PROVIDER mtplx`, `CONTEXT 128k`, and
  `PARALLEL 4`
- **THEN** the printed command includes `--max-active-requests 4` and
  `--context-window 128000`, with neither value derived from the other

#### Scenario: No PARALLEL means no change

- **WHEN** a Spinloop states no `PARALLEL`, for any of the four engines
- **THEN** the printed command is identical to what it would have been before
  this capability existed

#### Scenario: Invalid parallel count

- **WHEN** a Spinloop states `PARALLEL 0`, a negative number, or a non-numeric
  value, for any of the four engines
- **THEN** the command fails naming the invalid value rather than passing it
  to the engine

#### Scenario: Spinloop PARALLEL overrides a preset's own value

- **WHEN** a Spinloop states `PROVIDER llamacpp` and `PARALLEL 2`, and its
  `PRESET` section separately sets `np = 4`
- **THEN** the printed command includes `--parallel 2`, not `--parallel 4`

## ADDED Requirements

### Requirement: PARAM instructions

A Spinloop's `PARAM` instructions SHALL be one layer of the engine's flags,
rendered in the engine's own dialect exactly as preset keys are rendered. The
layers merge so that a PARAM stands over a preset's values and under the
Spinloop's own instructions (`MODEL`, `ALIAS`, `CONTEXT`, `PARALLEL`,
`BASEURL`): a typed instruction beats a raw pass-through, which beats the
preset. Where the same key is set by several PARAM lines, the later line's
value SHALL stand.

#### Scenario: A PARAM overrides the preset

- **WHEN** a preset section sets `temp = 0.7` and the Spinloop says
  `PARAM temp = 1.0`
- **THEN** the command carries a single temperature flag with value 1.0

#### Scenario: An instruction overrides a PARAM

- **WHEN** the Spinloop says `CONTEXT 32768` and `PARAM ctx-size = 5000`
- **THEN** the command carries a context size of 32768, unscaled

#### Scenario: PARAM booleans render as the engine's dialect allows

- **WHEN** the Spinloop says `PARAM jinja = 1` and `PARAM no-mmproj = 1` for
  llamacpp
- **THEN** the command includes a bare `--jinja` and a bare `--no-mmproj`

#### Scenario: A PARAM's context is not rescaled

- **WHEN** a Spinloop states `PROVIDER llamacpp`, `PARALLEL 2`, and
  `PARAM ctx-size = 4096`, and states no `CONTEXT`
- **THEN** the printed command includes `--parallel 2` and the PARAM's
  `ctx-size` value passes through unscaled

#### Scenario: A PARAM's parallelism loses to the Spinloop's

- **WHEN** a Spinloop states `PROVIDER llamacpp`, `PARALLEL 2`, and
  `PARAM np = 4`
- **THEN** the printed command includes `--parallel 2`, not `--parallel 4`

### Requirement: A PARAM can name the model

For an engine that takes its model as a flag — `llamacpp` and `mtplx` — a PARAM
naming the model SHALL satisfy serve's model requirement in place of a `MODEL`,
the way a preset's model reference does, and the model SHALL be passed to the
engine as that flag, unaltered. `vllm` takes its model positionally and SHALL
still require a `MODEL`; a PARAM cannot stand in for it.

#### Scenario: A PARAM names the model

- **WHEN** the Spinloop says `PROVIDER llamacpp`, `ALIAS qwen3.8-27b`, and
  `PARAM hf = unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL`, with no `MODEL` and no
  `PRESET`
- **THEN** the command carries the repository reference as the model flag, and
  serve does not fail for want of a model

#### Scenario: vLLM still needs a MODEL

- **WHEN** the Spinloop says `PROVIDER vllm` with PARAMs but no `MODEL`
- **THEN** the command fails explaining serve needs a MODEL, which vLLM takes
  positionally
