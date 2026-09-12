## Why

Hugging Face is where local models actually come from, but `spinloop` knows
nothing about it. Writing a Spinloop for one is a manual research job: open the
model page, work out whether it is GGUF, MLX or safetensors (which decides the
`PROVIDER`), scroll the file list for the quantisation you want, find
`max_position_embeddings` in `config.json` for the `CONTEXT`, and invent an
`ALIAS`. Get any of it wrong and the first thing you learn is that `serve`
fails, or that a 40 GB download has started for a quant you did not want.

Every fact needed to write that file is already published by the Hub — and for
a model you have downloaded before, it is already on your disk in the Hugging
Face cache. This change reads both and writes the Spinloop for you.

## What Changes

- **`spinloop hf <ref>`**: a new command that turns a Hugging Face model
  reference into a Spinloop. It inspects the repo, infers the `PROVIDER`,
  `MODEL`, `ALIAS` and `CONTEXT`, prints the file to stdout, and says what it
  inferred and why on stderr.
- **References are written the way people paste them**: `org/model`,
  `org/model:QUANT`, `hf.co/org/model`, a full `https://huggingface.co/...`
  URL (including a `/tree/<rev>` or `/blob/<rev>/<file>` suffix), and an
  explicit `@revision`.
- **The provider is inferred from the repo's files**: `*.gguf` means
  `llamacpp`, an MLX repo means `omlx`, plain safetensors means `vllm`.
  `--provider`/`-p` overrides it, and a repo that fits none of them fails
  saying so rather than guessing.
- **Quantisation selection**: for a GGUF repo, the `:QUANT` suffix (or
  `--quant`/`-q`) picks the file; with neither, a preference order picks a
  sensible default and the alternatives are listed. An ambiguous or unmatched
  quant fails listing what the repo actually offers.
- **The existing Hugging Face cache is used, never bypassed**: `spinloop hf`
  looks the model up in the local HF cache (`HF_HOME`/`HF_HUB_CACHE`, else
  `~/.cache/huggingface/hub`) and in llama.cpp's own cache. A cached GGUF
  becomes a `MODEL` pointing at the file on disk, so the engine does not
  download a second copy; anything already cached also means the metadata can
  be read locally, so a fully cached model resolves with no network at all.
- **Optional Hugging Face token**: a token is resolved from `HF_TOKEN`, then
  `HUGGING_FACE_HUB_TOKEN`, then the CLI's stored token file, and sent as a
  bearer only when one exists — so gated and private repos work for a logged-in
  user, and everyone else is unaffected. The token is never written to disk or
  printed.
- **`--output-file`/`-o` writes the Spinloop** instead of printing it, refusing
  to overwrite an existing file without `--force`.
- **`--apply` dresses the harness straight away**, going through the same path
  `spinloop apply` uses, so `spinloop hf <ref> --apply` is the one-liner from model
  page to working agent.
- **No weights are downloaded by `spinloop`.** It reads the cache and the Hub's
  metadata; fetching weights stays the engine's job, as it is today.
- **No new module dependency**: the Hub client is a small leaf package using
  the two JSON endpoints and the documented cache layout, in keeping with a CLI
  whose whole point is having nothing to install.

## Capabilities

### New Capabilities

- `huggingface-hub`: reading Hugging Face — parsing a model reference in its
  several written forms, fetching repo metadata and `config.json` over the
  Hub's API, resolving an optional token, locating an already-cached repo in
  the HF and llama.cpp caches, and the bounded, quiet failure behaviour that
  keeps a slow or unreachable Hub from hanging or breaking a command.
- `huggingface-spinloops`: the `spinloop hf` command — how a resolved repo becomes
  a `Selection` (provider inference, quantisation choice, context and alias
  derivation, cached-file preference), the flags that override each inference,
  and how the result is printed, written or applied.

### Modified Capabilities

_None._ The Spinloop file format, `apply`, `serve` and completion all consume the
result exactly as they already do: `spinloop hf` emits an ordinary Spinloop, and a
cached-file `MODEL` is the local-path form `serve` already understands.

## Impact

- New package `internal/hf`: reference parsing, the Hub metadata client, token
  resolution, cache lookup, and the inference rules that turn a repo into a
  `Selection`. A leaf package — stdlib only, importing nothing of ours but
  `internal/contextsize`.
- New `cmd/spinloop/hf.go` holding the `hfCmd` constructor and its flag set,
  added to `newRootCmd`'s `AddCommand` list in `commands.go`.
- `cmd/spinloop/complete.go` supplies what the tree cannot: provider-name
  completion for `-p` (the `compProviders` `add` uses), a file completion for
  `-o`, and a no-candidate slot for the reference; `TestCompletionCoversTree`
  walks the tree so the completions cannot drift from it.
- Docs: a new `docs/commands/hf.md`, entries in `docs/README.md` and
  `docs/env-vars.md` (`HF_TOKEN`, `HF_HOME`, `HF_HUB_CACHE`, `LLAMA_CACHE`),
  a README quickstart line, and an `AGENTS.md` layout entry.
- Network: a second outbound host beyond the AWS control plane and provider
  endpoints — `huggingface.co`, over the same best-effort, timeout-bounded
  discipline `internal/discovery` already follows.
- No change to any harness adapter, to the catalogue, or to the Spinloop format.
