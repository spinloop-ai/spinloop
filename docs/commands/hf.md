# spinloop hf

Write the [`Spinloop` file](../spinloop-file.md) for a model on the Hugging Face
hub, from the reference you'd paste anyway:

```sh
spinloop hf unsloth/Qwen3.6-35B-A3B-GGUF
spinloop hf unsloth/Qwen3.6-35B-A3B-GGUF -o ./Spinloop
spinloop hf https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF --apply
```

It reads the repo — metadata only, never the weights — and prints the Spinloop
that serves it:

```
PROVIDER llamacpp
MODEL    unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M
ALIAS    qwen3.6-35b-a3b
CONTEXT  262144
```

What was inferred, and from what, is said on stderr — the provider and why,
the quantisation and the alternatives, the context and its source, and which
copy of the model was used — so stdout stays a clean Spinloop you can pipe
anywhere.

## The reference

Any of these name the same thing:

```
unsloth/Qwen3.6-35B-A3B-GGUF
unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M
hf.co/unsloth/Qwen3.6-35B-A3B-GGUF
huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF
https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF
https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main
https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/blob/main/Qwen3.6-35B-A3B-Q4_K_M.gguf
unsloth/Qwen3.6-35B-A3B-GGUF@main
```

The `:QUANT` suffix picks the quantisation, and `@revision` (or a
`/tree/<revision>` in the URL) picks a non-default revision. A `/blob/…/file`
URL names one file; its quantisation is read off the filename.

## What it infers, and how to override it

| Value | Inferred | Override |
| ----- | -------- | -------- |
| `PROVIDER` | From the repo's files: a `.gguf` means `llamacpp`, a repo published for MLX means `omlx`, plain `.safetensors` means `vllm` | `-p`, `--provider` |
| `MODEL` | The repo reference, with the chosen quantisation when the engine picks a file | `-q`, `--quant` or a `:QUANT` suffix |
| `ALIAS` | The repo name, lower-cased, with a packaging suffix such as `-GGUF` or `-MLX` removed | `-a`, `--alias` |
| `CONTEXT` | The window the model's own `config.json` declares | `-c`, `--context` (the same lenient sizes: `32k`, `1.5m`, …) |

A repo holding none of the recognised file kinds fails, saying what it appears
to hold and which providers could be inferred. `-p` is checked against the
catalogue up front, before anything is fetched.

`OUTPUT` is never written: applying a Spinloop already defaults the output to
a quarter of the context, and `hf` writes nothing of the sort.

### Quantisations

The choice, in order: the reference's `:QUANT` suffix, then `-q`, then a
documented preference for a mid-sized K-quant — `Q4_K_M`, `UD-Q4_K_XL`,
`Q4_K_S`, `Q5_K_M`, `Q6_K`, `Q8_0`, falling back to the smallest
non-full-precision quantisation, with the name as tie-break. The narration
names the choice and lists the alternatives, and a quantisation the repo does
not have fails listing the ones it does. A quantisation published as numbered
shards is one choice, and the `MODEL` written loads the whole set.

## The caches

Before it asks the hub anything, it looks at the two caches a model may already
be in — the Hugging Face hub cache (`$HF_HUB_CACHE`, else `$HF_HOME/hub`, else
`~/.cache/huggingface/hub`) and llama.cpp's cache (`$LLAMA_CACHE`, else
`~/.cache/llama.cpp`) — so a model you have already downloaded resolves
offline. Where the chosen quantisation is on disk, the `MODEL` is that file's
path and the narration names the cache it came from; the engine then loads
what is there instead of downloading a second copy.

The caches are separate: a model `llama-server` downloaded into llama.cpp's
cache is not in the Hugging Face cache, and one pulled with the hub's tools is
the other way round. `hf` checks both.

Because a path names one machine's disk, `--no-cache` writes the repo reference
even when a copy is cached — for a Spinloop meant to be committed and shared —
and the narration notes that the cached copy stays on this machine. The engines
that load a repo rather than a single weights file (`omlx`, `vllm`) always get
the repo reference.

`spinloop hf` never downloads weights. Reading a repo is metadata only, and a
command that describes a model never begins a multi-gigabyte transfer as a
side effect.

## Authentication

Public repos need nothing. For gated or private ones, a token is sent as a
bearer when one is found, in this order: `HF_TOKEN`, then
`HUGGING_FACE_HUB_TOKEN`, then the file `$HF_HOME/token` (else
`~/.cache/huggingface/token`) — the same places the hub's own tools read. A
gated repo with no usable token fails saying to set `HF_TOKEN` or log in with
the Hugging Face CLI.

`HF_ENDPOINT` points the metadata calls at a mirror or a test server; the
default is `https://huggingface.co`.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-p`, `--provider` | Engine to serve it with (default: inferred from the repo's files) |
| `-q`, `--quant` | Quantisation to serve (default: the preference order, alternatives named on stderr) |
| `-c`, `--context` | Context window (default: the window the model's config declares) |
| `-a`, `--alias` | Short name for the model (default: the repo name, minus a packaging suffix) |
| `-o`, `--output-file` | Write the Spinloop to this file instead of stdout |
| `--force` | Overwrite an existing `--output-file` |
| `--no-cache` | Write the repo reference even when a copy is cached, so the Spinloop stays portable |
| `--apply` | Configure the active harness from the result |
| `-H`, `--harness` | Which harness to configure (with `--apply`) |

## Notes

- `-o` here names the **output file**. On [`add`](add.md) and
  [`apply`](apply.md) the same shorthand is the max **output tokens** — a
  different flag on a different command, and the reason this one spells out
  `--output-file` in its help.
- An existing `--output-file` is not overwritten unless `--force` is given, so
  a hand-edited Spinloop cannot be lost to a mistyped command.
- `--apply` configures the harness by the same path `spinloop apply` uses, so
  one command goes from a model page to a dressed agent.

## See also

- [`spinloop apply`](apply.md) — apply a `Spinloop` file you already have
- [`spinloop serve`](serve.md) — run the engine the Spinloop names
- [`spinloop list`](list.md) — the catalogue of providers
- [Environment variables](../env-vars.md) — `HF_TOKEN` and friends
