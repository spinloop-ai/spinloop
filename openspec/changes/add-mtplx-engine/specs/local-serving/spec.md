## MODIFIED Requirements

### Requirement: Choosing the engine

`spinloop serve` SHALL launch the inference engine the Spinloop's `PROVIDER` names,
the local counterpart of the runner `spinloop remote deploy` selects from the same
instruction. `llamacpp` SHALL run `llama-server`; `omlx` SHALL run the oMLX CLI;
`vllm` SHALL run `vllm serve`, with the model passed as its positional
argument, the served name as `--served-model-name`, and the context window as
`--max-model-len`; `mtplx` SHALL run `mtplx serve`, with the model as
`--model`, the served name as `--model-id`, the context window as
`--context-window`, and a `--download` flag so the engine fetches a model it
does not have itself. There SHALL be no default: a `PROVIDER` that is not a
self-hosted engine SHALL fail, naming the providers that can be served, rather
than launching an engine the Spinloop did not ask for.

An engine whose executable is normally installed outside the `PATH` SHALL also be
looked for at its conventional install location, so a user who has never put it
on their `PATH` can still serve.

#### Scenario: Provider selects the engine

- **WHEN** the Spinloop says `PROVIDER omlx`
- **THEN** the printed command runs the oMLX CLI, not `llama-server`

#### Scenario: vLLM is servable

- **WHEN** the Spinloop says `PROVIDER vllm` with a `MODEL`
- **THEN** the printed command runs `vllm serve` with that model as the
  positional argument

#### Scenario: MTPLX is servable

- **WHEN** the Spinloop says `PROVIDER mtplx` with a `MODEL` naming an MTPLX
  optimised repo or a local path
- **THEN** the printed command runs `mtplx serve` with that model as `--model`
  and includes `--download`

#### Scenario: MTPLX served name and context

- **WHEN** the Spinloop says `PROVIDER mtplx` with `ALIAS qwen` and
  `CONTEXT 128k`
- **THEN** the printed command carries `--model-id qwen` and
  `--context-window 131072`

#### Scenario: A provider that is not a local engine

- **WHEN** the Spinloop names a hosted provider such as `ollama` or `openrouter`
- **THEN** the command fails, listing the providers that can be served locally

#### Scenario: Engine installed outside the PATH

- **WHEN** the oMLX CLI is not on the `PATH` but is present in its macOS app
  bundle
- **THEN** `serve` uses the bundled executable

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
daemon-pushed config, or `remote deploy`); a value that is not SHALL fail
naming the invalid value rather than being passed to the engine.

When `CONTEXT` is supplied by a `PRESET` section's own `ctx-size` rather than
by the Spinloop, `PARALLEL` SHALL NOT rescale it — only a Spinloop-stated
`CONTEXT` participates in the `llamacpp` multiply. A `PARALLEL` value SHALL
still override a preset's own `np`/`parallel` (llama.cpp), `max-num-seqs`
(vLLM), `max-concurrent-requests` (oMLX), or `max-active-requests` (MTPLX)
value by the same override-by-canonical-name rule `CONTEXT` already uses
against a preset's `ctx-size`.

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
  `--context-window 131072`, with neither value derived from the other

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
