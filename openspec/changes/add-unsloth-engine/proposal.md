# Add Unsloth Studio engine support

## Why

[Unsloth Studio](https://unsloth.ai) installs an `unsloth` CLI whose
`unsloth run` loads one model at start and serves it through an
OpenAI-compatible API (`/v1/chat/completions`, `/v1/models`, …), driving its
own llama.cpp build underneath with Unsloth's GPU-fitting and speculative
decoding defaults. Users who already have Studio installed cannot point a
Spinloop at it: the engine table only knows `llamacpp`, `omlx`, `mtplx` and
`vllm`, and the catalogue has no `unsloth` provider, so `PROVIDER unsloth`
can be neither served nor applied to a harness.

## What Changes

- `unsloth` becomes a servable engine. `PROVIDER unsloth` runs
  `unsloth run`, with the model as `--model` (an HF repo, optionally
  `org/repo:variant`, or a local path, passed verbatim), the context window
  as `--context-length`, and `BASEURL`'s host and port as `--host`/`--port`.
  `--api-only` (no web UI) and `--disable-tools` (no server-side web search
  or code execution) are always passed, the same defaults Unsloth's own
  `unsloth start` uses for a coding agent; a PARAM or preset can switch either
  back.
- For `unsloth`, `PARALLEL n` renders as `--parallel n`. Unsloth's slots
  share one unified KV cache of the full context, so `--context-length` is
  not scaled by `PARALLEL`.
- `ALIAS` has no server flag: Unsloth serves the model under its own id (the
  HF repo, or a local file's name), and ignores the request's model field
  for the loaded model, so the harness's `ALIAS` still reaches it.
- The `unsloth` CLI is looked for on the `PATH`, then at the Studio
  installer's own location (`~/.unsloth/studio/unsloth_studio/bin/unsloth`).
- The catalogue gains an `unsloth` provider: OpenAI-compatible, defaulting
  to `http://localhost:8888/v1`, overridable with `UNSLOTH_BASE_URL`,
  Pi-capable and lucinate-capable. Unsloth gates `/v1` with an API key of
  its own making (printed by `unsloth run` as it starts), so the key is read
  from `UNSLOTH_API_KEY` — the variable Unsloth's own client tooling reads.
  It is optional, because Studio's keyless-access setting removes the gate.
- The daemon can supervise `unsloth` through the same engine table. It
  cannot gate it with a spinloop-supplied key — `unsloth run` has no option
  to read one from a file — so a keyed start is refused, as for oMLX.
- Docs and an `examples/unsloth/` guide.

## Capabilities

### New Capabilities

(none — this change modifies existing capabilities only)

### Modified Capabilities

- `local-serving`: "Choosing the engine" — `unsloth` SHALL run
  `unsloth run` with the model as `--model`, the context window as
  `--context-length`, and `--api-only` and `--disable-tools` by default.
  "Parallelism" — for `unsloth`, `PARALLEL n` SHALL render as
  `--parallel n` without scaling the context. "A PARAM can name the model" —
  `unsloth` joins the engines that take their model as a flag.
- `provider-catalog`: a new "Unsloth Studio local provider" requirement —
  the catalogue SHALL include an `unsloth` provider with the plumbing above.

## Impact

- Go client: `cmd/spinloop/serve.go` (the engine table's `unsloth` entry
  and its serve params, binary lookup), `cmd/spinloop/cloud.go`
  (`modelSourceFlags`/preset key helpers gain the `unsloth` case),
  `internal/preset/preset.go` (the `Unsloth` dialect: short aliases and bare
  flags), `internal/catalog/providers.yaml` (the `unsloth` entry).
- Non-goals: fleet wake and cloud deploy for `unsloth` (no machine image;
  wake is a follow-up), metrics (Unsloth's `/metrics` passthrough needs the
  owner's key, which the daemon does not have), readiness (Unsloth has no
  `/health`; its `/api/health` reports liveness only, before the model is
  loaded), and `hf` inference choosing `unsloth` for a repo.
- Docs: `README.md`, `docs/commands/serve.md`, `docs/guides/local-serving.md`,
  `docs/spinloop-file.md`, `docs/env-vars.md`, `docs/index.md`,
  `docs/openapi.yaml` (runner examples), `examples/unsloth/`.
- Tests: the Go suite (engine table, flag rendering, binary lookup, the
  catalogue entry). Coverage stays at 80% or above.
