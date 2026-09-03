# Add mtplx engine support

## Why

MTPLX is an MLX-based inference server for Apple Silicon with native MTP
speculative decoding — for the optimised models it ships, it decodes several
times faster than other local engines, which is why Macs running coding
workloads increasingly use it. It is OpenAI-compatible and binds the model it
serves at start, which makes it a natural fit for spinloop: servable,
supervisable, and routable. Today a `PROVIDER mtplx` Spinloop is none of
those — the engine table knows only `llamacpp`, `omlx`, and `vllm`, the
catalogue has no `mtplx` entry, and the fleet cannot wake a node for it.

## What Changes

- `mtplx` becomes a servable engine. `PROVIDER mtplx` runs `mtplx serve`,
  with the model as `--model`, the served name as `--model-id`, the context
  window as `--context-window`, and a `--download` flag so the engine fetches
  a model it does not have itself, the way `llama-server -hf` does. MTPLX
  preset keys are all long-form, so presets pass through unchanged.
- For `mtplx`, `PARALLEL n` renders as `--max-active-requests n`, MTPLX's
  admission cap on concurrent requests. The scheduling mode — how admitted
  requests execute — stays a preset concern: `PARALLEL` never selects it.
- The fleet can wake a node for `mtplx`. The Spinloop-to-deploy-config
  derivation becomes runner-aware: the node path accepts `mtplx` alongside
  `llamacpp` and `vllm`, per-engine preset fallback keys replace the
  llama.cpp-only ones, and a `MODEL` naming a file on the node's own disk is
  a valid wake — today the local-path check fires even on the node path,
  which also blocks llamacpp and vllm wakes with local weights. Cloud deploy
  is unchanged: MTPLX is Apple-Silicon-only and has no machine image, so
  `remote deploy` still refuses it.
- Readiness: the daemon gains a `/health` convention for `mtplx`, so a
  started or woken node reports readiness the way llamacpp and vllm do.
- The catalogue gains an `mtplx` provider: an OpenAI-compatible endpoint
  defaulting to `http://localhost:8000/v1`, overridable with
  `MTPLX_BASE_URL`, optionally authenticated, Pi-capable, and
  lucinate-capable.
- MTPLX exposes no Prometheus metrics — its `/metrics` is a JSON ring of
  per-request envelopes — so it gets the omlx treatment: no scrape dialect,
  no token stats, nothing sampled.
- Docs and an `examples/mtplx/` guide.

## Capabilities

### New Capabilities

(none — this change modifies existing capabilities only)

### Modified Capabilities

- `local-serving`: "Choosing the engine" — `mtplx` SHALL run `mtplx serve`
  with the model as `--model`, the served name as `--model-id`, the context
  window as `--context-window`, and `--download` so a missing model is
  fetched by the engine. "Parallelism" — for `mtplx`, `PARALLEL n` SHALL
  render as `--max-active-requests n`, an admission cap that does not scale
  the context; `PARALLEL` never selects MTPLX's scheduling mode.
- `fleet-routing`: "Waking a node" — a node may be woken for an engine that
  binds its model at launch (`llamacpp`, `vllm`, `mtplx`), and a `MODEL`
  naming a file on the node's own disk is a valid wake for it.
- `provider-catalog`: a new "MTPLX local provider" requirement — the
  catalogue SHALL include an `mtplx` provider with the plumbing above.

## Impact

- Go client: `cmd/spinloop/serve.go` (the engine table's `mtplx` entry),
  `cmd/spinloop/remote.go` (the runner-aware derivation: the node gate,
  per-engine preset fallback keys, the local-path check moving to the
  cloud-only path, the owned preset keys), `internal/daemon/readiness.go`
  (the `/health` convention), `internal/catalog/providers.yaml` (the
  `mtplx` entry). No `internal/preset` change — MTPLX's dialect is the zero
  value.
- Non-goals: metrics (no Prometheus dialect), cloud deploy (no machine
  image), and omlx fleet wake (tracked separately, #159).
- Docs: `docs/commands/serve.md`, `docs/spinloop-file.md`,
  `docs/commands/remote.md`, `docs/openapi.yaml` (the runner examples),
  `examples/mtplx/`.
- Tests: the Go suite (the engine table, preset rendering, readiness,
  per-runner deploy-config derivation, the catalogue entry). Coverage stays
  at 80% or above.
