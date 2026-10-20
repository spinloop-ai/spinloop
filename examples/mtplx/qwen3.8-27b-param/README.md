# Qwen3.8-27B on MTPLX — PARAMs, no preset

Run an MTPLX-optimised build of Qwen3.8-27B on your Mac with
[MTPLX](https://mtplx.com), then point opencode at it with the
[`Spinloop`](Spinloop) in this directory.

What makes this example different from its
[`qwen3.8-27b`](../qwen3.8-27b/README.md) sibling: there is no
`preset.ini`. The engine options live in the Spinloop itself, one `PARAM` per
flag — and `PARAM model` names the model in place of a `MODEL`:

```dockerfile
PROVIDER mtplx
ALIAS    qwen3.8-27b
CONTEXT  32768
PARALLEL 4
PARAM model = Youssofal/Qwen3.8-27B-MTPLX-Optimized-Speed
PARAM scheduler-mode = ar_batch
```

MTPLX takes its model as a flag, so a PARAM with the key `model` stands in
for `MODEL`: `serve` reads it the way it reads a preset's `model`, so a
Spinloop with no preset file at all still serves its model. MTPLX's
long-form keys pass through unchanged — there is no llama.cpp-style alias to
learn.

MTPLX is a single-model server: you launch it with one model and it serves
that model over an OpenAI-compatible API. That makes it the closest of the
local engines to
[llama.cpp](../../llamacpp/qwen3.8-27b/README.md), but tuned for Apple
Silicon unified memory.

## Prerequisites

- An **Apple Silicon** Mac (M1 or later). MTPLX is Apple Silicon only — there
  is no Intel or Linux build, and **no machine image**, so
  [`spinloop cloud`](../../../docs/commands/cloud.md) cannot deploy it. It
  serves locally, or on a
  [fleet node](../../../docs/commands/fleet.md) you run yourself.
- [MTPLX](https://mtplx.com), installed so that `mtplx` is on your `PATH`.
- Enough unified memory for the weights.

## 1. Start MTPLX

```sh
spinloop serve              # from this directory; reads ./Spinloop
spinloop serve --dry-run    # print the `mtplx serve` command without running it
```

`spinloop serve` builds the command from the Spinloop, so it runs
equivalently to:

```sh
mtplx serve \
  --model Youssofal/Qwen3.8-27B-MTPLX-Optimized-Speed \
  --scheduler-mode ar_batch \
  --download \
  --model-id qwen3.8-27b \
  --context-window 32768 \
  --max-active-requests 4
```

The command omits `--host`/`--port` because MTPLX's defaults (127.0.0.1:8000)
stand; the file ships a commented-out `BASEURL` line to change both what
opencode calls and what `serve` binds to.

What the PARAMs do:

- `model` — the Hugging Face repo to serve, or a local path, taken verbatim.
  A PARAM may name the model for an engine whose model is a flag, and MTPLX's
  is.
- `scheduler-mode = ar_batch` — how admitted requests execute. `spinloop`
  passes the value through without checking it, so it must be valid for your
  `mtplx` build (`mtplx serve --help` lists the current modes). This is
  per-deployment tuning, not a Spinloop field.
- `CONTEXT 32768` — the context a single request gets, rendered as
  `--context-window`. Never scaled by `PARALLEL`.
- `PARALLEL 4` — an admission cap on how many requests run at once, rendered
  as `--max-active-requests`.

`spinloop serve` always passes `--download` — fetch the repo if it is not
already local, instead of failing the launch — and `--model-id`, the name the
model is served under (the `ALIAS`).

### Check it's up

```sh
curl http://127.0.0.1:8000/v1/models
curl http://127.0.0.1:8000/health
```

`/health` answers once the OpenAI server is up, which may be before the weights
finish loading.

## 2. Point opencode at it

MTPLX speaks the OpenAI-compatible API, which is what the `mtplx` provider
targets (default base URL `http://localhost:8000/v1`, overridable with
`MTPLX_BASE_URL`). Apply the [`Spinloop`](Spinloop) in this directory:

```sh
spinloop harness apply examples/mtplx/qwen3.8-27b-param/Spinloop
# or, from this directory:
spinloop harness apply
```

`ALIAS` is the name opencode shows for the model, and `CONTEXT` matches
opencode's context window to the `--context-window` the server was launched
with. Now start `opencode` and select `mtplx/qwen3.8-27b`.

## A note on API keys

By default `spinloop` writes no key for a localhost MTPLX: the `mtplx` provider
is marked `apiKeyOptional`, so a local endpoint with no `OPENAI_API_KEY` set
gets no `apiKey` field at all. If you put a key on the engine, set
`OPENAI_API_KEY` **before** you apply the Spinloop, and `spinloop` writes an
`{env:OPENAI_API_KEY}` reference (never the secret).

`spinloop serve` never passes `--api-key`: it prints the command it runs, so a
key there would land on your screen and in the process table. A supervised
engine (`serve --api` or `spinloop daemon`) is gated with a key file the daemon
writes instead.

## See also

- The same model, its engine options in a `preset.ini`:
  [`examples/mtplx/qwen3.8-27b`](../qwen3.8-27b/README.md)
- The same model on vLLM:
  [`examples/vllm/qwen3.8-27b`](../vllm/qwen3.8-27b/README.md)
- [`spinloop serve`](../../../docs/commands/serve.md) — the full command
  reference
- [The `Spinloop` file](../../../docs/spinloop-file.md) — full syntax
