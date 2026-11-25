# Qwen3.8-27B on Unsloth Studio

Serve Unsloth's own GGUF of Qwen3.8-27B with
[Unsloth Studio](https://unsloth.ai), then point opencode at it with the
[`Spinloop`](Spinloop) in this directory.

Studio's `unsloth run` loads one model and serves it over an
OpenAI-compatible API. It runs Studio's own llama.cpp build with Unsloth's
GPU-fitting and speculative-decoding defaults, so it serves the same GGUF files
as [llama.cpp](../../llamacpp/qwen3.8-27b/README.md), but you don't have to tune
the flags yourself.

## Prerequisites

- [Unsloth Studio](https://unsloth.ai), desktop app or installer. Either one
  installs the `unsloth` command. `spinloop` finds it on your `PATH`, or at
  `~/.unsloth/studio/unsloth_studio/bin/unsloth` if it is not on your `PATH`.
- Enough GPU or unified memory for the weights (about 17 GB for `UD-Q4_K_XL`).

## 1. Start Studio

```sh
spinloop serve              # from this directory; reads ./Spinloop
spinloop serve --dry-run    # print the command without running it
```

The command it runs:

```sh
unsloth run --api-only --disable-tools \
  --model unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL \
  --context-length 64000 --parallel 2 \
  --host 127.0.0.1 --port 8899
```

- `--model` takes the Hugging Face repo, with `:VARIANT` picking the quant.
  Studio downloads it on first use.
- `--api-only` serves only the API, with no web UI. `--disable-tools` turns
  off Studio's server-side web search and code execution. `spinloop serve`
  always passes both. Put `PARAM disable-tools = false` in the Spinloop if you
  want the tools.
- `--context-length` is the context a single request gets. Studio may lower
  it to fit your GPU memory. The startup banner shows the length in use.
- `--parallel` is the number of decode slots. The slots share one KV cache,
  so the context is not multiplied by the slot count.
- `--host`/`--port` come from `BASEURL`. This example uses `8899` because the
  Unsloth desktop app already listens on Studio's default `8888`. If the port
  is taken, Studio moves to the next free one without failing.

When the model has loaded, Studio prints a banner with the URL and a
`Bearer sk-unsloth-…` API key.

## 2. Point opencode at it

Studio requires that key on every `/v1` request. It also stores the key
under `~/.unsloth/studio/auth/` and reuses it on every run, and spinloop reads
that copy when `UNSLOTH_API_KEY` is unset. So launching the agent needs no
export:

```sh
spinloop code examples/unsloth/qwen3.8-27b/Spinloop
```

The harness config stores a reference to `UNSLOTH_API_KEY`, never the key
itself, and `spinloop code` starts the agent with the variable set. To run
opencode yourself instead, export the key from the banner first:

```sh
export UNSLOTH_API_KEY=sk-unsloth-…   # from the banner
spinloop harness apply examples/unsloth/qwen3.8-27b/Spinloop
```

You can also turn on keyless access in Studio's settings.

### Check it's up

```sh
curl -H "Authorization: Bearer $UNSLOTH_API_KEY" http://127.0.0.1:8899/v1/models
```

`/v1/models` lists every model Studio has downloaded, and the one in memory
has `"loaded": true`.

## About `ALIAS`

Studio has no flag for a served name. It lists the model as
`unsloth/Qwen3.8-27B-GGUF`, but answers a chat request with the loaded model
whatever name the request carries. So opencode can still ask for
`qwen3.8-27b`. Don't choose an `ALIAS` that is the repo id of a *different*
model you have downloaded, because Studio may try to switch to that model.

## See also

- [`spinloop serve`](../../../docs/commands/serve.md#unsloth-studio) — the Unsloth section of the command reference
- [The `Spinloop` file](../../../docs/spinloop-file.md) — full syntax
