# Qwen3.6-35B-A3B on oMLX — PARAMs, no preset

Run an MLX build of Qwen3.6-35B-A3B on your Mac with [oMLX](https://omlx.ai),
then point opencode at it with the [`Spinloop`](Spinloop) in this directory.

What makes this example different from its
[`qwen3.6`](../qwen3.6/README.md) sibling: there is no `preset.ini`. The
engine options live in the Spinloop itself, one `PARAM` per flag:

```dockerfile
PROVIDER omlx
MODEL    Qwen3.6-35B-A3B-4bit
CONTEXT  32768
PARAM model-dir = /Users/you/models
PARAM memory-guard = safe
PARAM paged-ssd-cache-dir = /Users/you/.omlx/cache
PARAM max-concurrent-requests = 8
```

Paths are passed to the server verbatim, so `~` is not expanded — spell them
out in full, the way the sibling's preset does.

oMLX is built on [MLX](https://github.com/ml-explore/mlx), Apple's array
framework, so it runs on the GPU and Neural Engine rather than through a CUDA
path. Its headline feature is **paged SSD caching**: KV-cache blocks are
persisted to disk, so returning to a long prompt restores instead of recomputing
it. For a coding agent, which re-sends a large and mostly-unchanged context on
every turn, that is the difference between a usable and an unusable setup — and
it is one of the PARAMs in this Spinloop.

## Prerequisites

- An **Apple Silicon** Mac (M1 or later). oMLX is Apple Silicon only — there is
  no Intel or Linux build, and no cloud equivalent, so
  [`spinloop cloud`](../../../docs/commands/cloud.md) cannot deploy it.
- [oMLX](https://omlx.ai), installed from its DMG or from source.
- Enough unified memory for the weights. The 4-bit build is roughly 20 GB, so
  plan for a 32 GB machine or better.

## 1. Pull the model

oMLX's admin panel (`http://localhost:8000/admin`) has a Hugging Face browser
that downloads a model in one click, which is the easiest route.

To do it from the shell instead, put the model in the directory oMLX serves
from — the Spinloop's `model-dir` PARAM, here `/Users/you/models`:

```sh
hf download mlx-community/Qwen3.6-35B-A3B-4bit \
  --local-dir /Users/you/models/Qwen3.6-35B-A3B-4bit
```

The **directory name** is what the model is called over the API —
`Qwen3.6-35B-A3B-4bit` here — unless you set an alias for it in the admin panel.
That is the name the `Spinloop` uses as its `MODEL`.

## 2. Start oMLX

```sh
spinloop serve              # from this directory; reads ./Spinloop
spinloop serve --dry-run    # print the omlx-cli command without running it
```

`spinloop serve` builds the command from the Spinloop, so it runs
equivalently to:

```sh
/Applications/oMLX.app/Contents/MacOS/omlx-cli serve \
  --model-dir /Users/you/models \
  --memory-guard safe \
  --paged-ssd-cache-dir /Users/you/.omlx/cache \
  --max-concurrent-requests 8
```

`serve` finds `omlx-cli` on your `PATH`, falling back to the app bundle — so
this works even if you've only ever launched oMLX from the menu bar.

What the PARAMs do:

- `model-dir` — the directory of models to serve. oMLX loads *all* of them and
  picks per request, which is why there's no "the model" flag here — `MODEL`
  names what the *agent* asks for, and no PARAM can stand in for it.
- `memory-guard = safe` — keep memory headroom for the rest of the machine.
  `balanced` gives the model more; `--memory-guard-gb` sets a ceiling directly.
- `paged-ssd-cache-dir` — persist KV-cache blocks to SSD. This is the setting
  that makes long agent contexts fast on repeat turns.
- `max-concurrent-requests = 8` — continuous batching width (the default).
- `CONTEXT 32768` — opencode's context window. Unlike llama.cpp there is no
  `--ctx-size` to keep it in step with — oMLX sizes its cache dynamically.

Note these are `omlx-cli` flags, **not** llama.cpp's. Each engine reads its
PARAMs in its own vocabulary, so these lines and the ones under
[`examples/llamacpp/`](../../llamacpp/) are not interchangeable.

### Check it's up

```sh
curl http://127.0.0.1:8000/v1/models
```

This is also how to confirm what your model is actually called — the response
lists the names requests must use.

## 3. Point opencode at it

oMLX speaks the OpenAI-compatible API, which is what the `omlx` provider
targets (default base URL `http://localhost:8000/v1`). Apply the
[`Spinloop`](Spinloop) in this directory:

```sh
spinloop harness apply examples/omlx/qwen3.6-param/Spinloop
# or, from this directory:
spinloop harness apply
```

`MODEL` matters more here than it does for a single-model llama.cpp server:
oMLX serves everything in its model directory and dispatches on the requested
name, so it must match a model oMLX actually has — its directory name, or the
alias you set in the admin panel. This Spinloop names the model downloaded
above; **if you downloaded something else, change `MODEL` to match.** List
what yours exposes with:

```sh
curl -H "Authorization: Bearer $OPENAI_API_KEY" http://localhost:8000/v1/models
```

A name oMLX does not have comes back as `model not found`. And unlike the
llama.cpp examples — where `spinloop serve` can fetch the weights from Hugging
Face — oMLX only serves what you have already downloaded.

Running on a different host or port? Add a `BASEURL` line (the file ships one
commented out):

```dockerfile
BASEURL http://127.0.0.1:9100/v1
```

That sets both what opencode calls and what `serve` binds to.

## A note on API keys

By default `spinloop` writes no key for a localhost oMLX: the `omlx` provider
is marked `apiKeyOptional`, so a local endpoint with no `OPENAI_API_KEY` set
gets no `apiKey` field at all.

**But oMLX can require a key even on localhost** — it is a per-install setting
in the admin panel, and some builds turn it on by default. If yours does,
requests without a key come back as `API key required`. To use it, set
`OPENAI_API_KEY` **before** you apply the Spinloop:

```sh
OPENAI_API_KEY=your-omlx-key spinloop harness apply     # writes an {env:OPENAI_API_KEY} reference
OPENAI_API_KEY=your-omlx-key opencode         # or `spinloop harness open`, which forwards it
```

The *before* matters: with the key set at apply time, `spinloop` writes an
`{env:OPENAI_API_KEY}` reference into the config (never the secret). With it
unset, `spinloop` omits the `apiKey` field entirely — and exporting the key
later does nothing, because there is no reference in the config to resolve. If
you hit `API key required` after the fact, set the variable and re-run
`apply`.

Reaching an oMLX on another machine works the same way: start it with
`--api-key`, export `OPENAI_API_KEY`, and point `OMLX_BASE_URL` at the Mac.

`spinloop serve` never passes `--api-key`: it prints the command it runs, so a
key there would land on your screen and in the process table. Configure auth in
oMLX itself.

## See also

- The same model, its engine options in a `preset.ini`:
  [`examples/omlx/qwen3.6`](../qwen3.6/README.md)
- The same model on llama.cpp:
  [`examples/llamacpp/qwen3.6-35b-a3b`](../../llamacpp/qwen3.6-35b-a3b/README.md)
- [`spinloop serve`](../../../docs/commands/serve.md) — the full command
  reference
- [The `Spinloop` file](../../../docs/spinloop-file.md) — full syntax
