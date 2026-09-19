# Serve a model locally

Run an inference engine on this machine from a [`Spinloop`
file](../spinloop-file.md), and point your coding agent at it. The file that
names the model is the same file that starts the server behind it — nothing
else to keep in step.

## 1. Install an engine

`spinloop` launches the engine; it does not ship one.

| `PROVIDER` | Engine to have installed |
| ---------- | ------------------------ |
| `llamacpp` | `llama-server` on your `PATH` — e.g. `brew install llama.cpp` |
| `omlx` | [oMLX](https://omlx.ai) on Apple Silicon |
| `vllm` | `vllm` on your `PATH` |
| `mtplx` | [MTPLX](https://mtplx.com) on Apple Silicon |

Any other provider in the catalogue names an endpoint somebody else runs, and
`serve` refuses it.

## 2. Write the Spinloop

```dockerfile
# Spinloop
PROVIDER llamacpp
MODEL    unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL   # an HF repo, or a .gguf path
ALIAS    qwen3.6
CONTEXT  32768
```

`MODEL` names a Hugging Face repo or a local weights file; `ALIAS` is the name
the model is served under; `CONTEXT` sizes the window each request gets. The
full field list is on [The `Spinloop` file](../spinloop-file.md). Have a model
page open instead? [`spinloop hf`](hugging-face.md) writes this file for you.

## 3. Start the server

```sh
spinloop serve            # reads ./Spinloop, prints the command, runs it
spinloop serve --dry-run  # print the command without running it
```

On a terminal, `serve` gives the engine a full-screen view — metrics above,
log below — and `q` stops the engine and exits. Off a terminal the engine's
output is forwarded as usual. `spinloop up` is the one-word form for
"start the server this directory holds".

## 4. Point the agent at it

```sh
spinloop harness apply    # merge the selection into the agent's config
spinloop code             # launch the agent against it
```

That is the whole loop: one file, two commands.

## Tuning

- **Full control** — for flags a Spinloop doesn't model (`-ngl`, KV-cache
  types, draft models), point `PRESET` at a preset file written in the
  engine's own flag vocabulary. The Spinloop's own fields win over the preset
  where they overlap. See [`spinloop serve`](../commands/serve.md#llamacpp).
- **Parallelism** — `PARALLEL` sets the number of concurrent request slots,
  translated per engine so `CONTEXT` keeps meaning *per request* everywhere.
  See [`spinloop serve`](../commands/serve.md#parallelism).
- **Supervised, not foreground** — [`spinloop daemon`](daemon.md) runs the
  same engine under the [HTTP control API](../http-api.md), so it can be
  started, stopped, and watched over HTTP instead of from a terminal.

## Where next

- [`spinloop serve`](../commands/serve.md) — the full reference, engine by
  engine
- [Run a daemon node](daemon.md) — the long-lived form
- [Runnable examples](https://github.com/spinloop-ai/spinloop/tree/main/examples)
  — ready-to-apply Spinloops with real models
