# Design

## What `unsloth run` is

`unsloth run` (an alias of `unsloth studio run`) starts Unsloth Studio's
FastAPI backend, loads the `--model` it is given through Studio's own
llama.cpp build, prints a banner (URL, API key, `Model loaded:`), and then
runs in the foreground until SIGINT/SIGTERM. On SIGTERM it shuts down its child
llama-server. With the default loopback bind and `--api-only` it never prompts
on stdin. These facts come from the Studio source (`unsloth_cli/commands/studio.py`,
`studio/backend/`) at version 2026.11.10.

It serves `/v1/chat/completions`, `/v1/completions`, `/v1/responses`,
`/v1/messages`, `/v1/models` and friends. `/v1/models` lists the loaded model
**and** every locally cached model, each with a `loaded` flag. The request's
`model` field is informational for the loaded model.

## The engine table entry

| Spinloop field | flag                       | notes                                              |
| -------------- | -------------------------- | -------------------------------------------------- |
| MODEL          | `--model`                  | verbatim: `org/repo`, `org/repo:VARIANT`, or a path |
| ALIAS          | —                          | Unsloth has no served-name flag                    |
| CONTEXT        | `--context-length`         | a token count, parsed as today                     |
| PARALLEL       | `--parallel`               | not scaled against CONTEXT (unified KV cache)      |
| BASEURL        | `--host` / `--port`        | omitted when unset; defaults `127.0.0.1:8888`      |
| (always)       | `--api-only`               | no web UI, no bootstrap password path              |
| (always)       | `--disable-tools`          | no server-side web search / code execution         |

- **Binary and subcommand.** `unsloth` with subcommand `run`. Lookup order:
  the package override (tests), the `PATH`, then
  `$HOME/.unsloth/studio/unsloth_studio/bin/unsloth`, the location the Studio
  installer writes to (it symlinks `~/.local/bin/unsloth` to it, but
  `~/.local/bin` is not always on the `PATH`). The fallback path is returned
  unchecked, like oMLX's bundle path, so a missing install surfaces as the
  usual not-found hint naming a concrete path.
- **`needsModel` is true** and `modelFlags` is `["model"]`. Without `--model`,
  `unsloth run` starts Studio with nothing loaded, which is not what a
  Spinloop asks for.
- **Defaults below everything else.** `--api-only` and `--disable-tools` are
  engine defaults, not typed instructions: they form the *lowest* layer of the
  merge, under the preset, so a preset or PARAM setting `api-only = false` or
  `disable-tools = false` drops them under the existing falsy-boolean rule. A
  new `defaults` field on `serveEngine` carries them; it is merged first in
  all three places that build a command (preset path, preset-less path, the
  daemon's deploy-config path). Every other engine has no defaults, so their
  commands are unchanged byte for byte. MTPLX's `--download` stays where it
  is; moving it is not this change's business.
- **Dialect.** A new `preset.Unsloth` dialect:
  - aliases: `hf`, `hf-repo` → `model`; `max-seq-length` → `context-length`;
    `np`, `n-parallel` → `parallel`; `H` → `host`; `p` → `port`;
    `q` → `silent`; `v` → `verbose`; `y` → `yes`.
  - boolean: `api-only`, `disable-tools`, `enable-tools`, `silent`,
    `verbose`, `yes`, and Studio's other on/off pairs (`load-in-4bit`,
    `no-load-in-4bit`, `tensor-parallel`, `no-tensor-parallel`,
    `cloudflare`, `no-cloudflare`, `secure`, `no-secure`,
    `enable-tool-call-healing`, `disable-tool-call-healing`,
    `enable-tool-call-nudging`, `disable-tool-call-nudging`,
    `disable-dns-pinning`).
  The aliases make the override-by-canonical-name rule hold however a preset
  spells a flag (`np = 4` is overridden by `PARALLEL 2`, `hf = …` names the
  model). Flags Studio does not know are forwarded by Studio to its
  llama-server, so preset keys like `ctx-size` or `jinja` still reach the
  engine.
- **No API key option.** `unsloth run` mints or reuses its own key and has no
  way to read one from a file, so `apiKeyFileFlag` is empty and a keyed daemon
  start is refused with the existing "cannot be gated" error, as for oMLX.
- **`defaultBaseURL` is `http://127.0.0.1:8888`** and `defaultBindLoopback`
  is true, so the daemon reports the right endpoint for a Spinloop with no
  `BASEURL`.
- **No metrics dialect.** Studio passes `/metrics` through to llama-server,
  but only for the owner's key, which the daemon does not hold. `metricsEngine`
  stays empty and `metricsArgs` nil — the omlx/mtplx treatment.
- **No readiness convention.** Studio answers 404 on `/health`; its
  `/api/health` is liveness only and is green before the model loads.
  `unsloth` is not added to `readinessCheckedRunners`, so its readiness field
  stays absent rather than claiming a ready engine that has not loaded.

## ALIAS

Unsloth serves the loaded model under its own id (the HF repo without the
variant, or a local `.gguf`'s file name). `ALIAS` cannot rename it, and the
harness config still uses `ALIAS` as the model id it sends. That works because
Studio answers a chat request with the loaded model whatever the request
names. The one exception in Studio's code is a request naming a *different
model that is downloaded locally*, which an alias is not. The docs say so and
recommend an `ALIAS` that is not another cached repo id.

## The catalogue entry

```yaml
unsloth:
  description: Unsloth Studio server (OpenAI-compatible API)
  name: Unsloth Studio
  npm: "@ai-sdk/openai-compatible"
  apiKeyEnv: UNSLOTH_API_KEY
  apiKeyOptional: true
  options:
    baseURL: http://localhost:8888/v1
  optionsFromEnv:
    baseURL: UNSLOTH_BASE_URL
  pi:
    api: openai-completions
  lucinate: {}
```

`UNSLOTH_API_KEY` is the variable Unsloth's own `unsloth start` reads, so a
user who already exports it for Unsloth's tooling has nothing more to set.
Studio requires a key by default; the key is optional in the catalogue because
Studio's keyless-access setting removes the requirement, and a local endpoint
with no key set must not stop `apply` from writing the provider.

## Non-goals

- **Fleet wake / cloud deploy.** `nodeRunnerFor` and `runnerFor` are
  unchanged. Wake needs a readiness signal that reflects a loaded model, and
  `/v1/models` listing cached-but-unloaded models would mislead routing that
  reads it. Both are follow-ups.
- **`hf` choosing `unsloth`.** `spinloop hf` keeps inferring `llamacpp` for a
  GGUF repo; a user who wants Studio sets `PROVIDER unsloth` themselves.
- **Reading the Studio key.** spinloop does not parse the banner or read
  Studio's key cache. The user exports `UNSLOTH_API_KEY` (or turns on keyless
  access), the same way they configure any other provider's key.
