# Tasks

## 1. Engine: the mtplx entry (cmd/spinloop/serve.go)

- [x] 1.1 Add the `mtplx` case to `engineFor`: `--model`, `--model-id`, `--context-window`, `--max-active-requests`, `--host`/`--port` (omitted when no BASEURL), `--download` always passed, `--api-key-file` key gating, `needsModel`, loopback default bind, default base URL `http://127.0.0.1:8000`
- [x] 1.2 Serve tests: model, alias, context, parallel, baseurl rendered; baseurl omitted entirely when unset; `--download` present; a set key never on the command line
- [x] 1.3 Confirm the zero preset dialect renders mtplx preset keys verbatim (long-form passthrough, no alias table)

## 2. Readiness (internal/daemon/readiness.go + the address plumbing)

- [x] 2.1 Add `mtplx` to `readinessCheckedRunners`, decoupling the engine's address from its metrics dialect so a dialect-less engine can still be probed at `/health` (see design, "Readiness")
- [x] 2.2 Tests: an open mtplx engine reports ready, a gated one (401) counts as ready, an unreachable one does not; the sampler skips a dialect-less target; `scrapeTargetFor` resolves an mtplx address with an empty dialect

## 3. Catalogue (internal/catalog/providers.yaml)

- [x] 3.1 Add the `mtplx` provider: `http://localhost:8000/v1` default, `MTPLX_BASE_URL` override, optional `OPENAI_API_KEY`, Pi `openai-completions`, the lucinate marker
- [x] 3.2 Tests: opencode with no key (no `apiKey` option), opencode with a key (environment reference, never the secret), Pi keyless placeholder, lucinate accepted

## 4. Wake: the runner-aware deploy-config path (cmd/spinloop/remote.go)

- [x] 4.1 Move the runner gate to the deploy target: the cloud keeps `llamacpp`/`vllm` and its error message; the node path accepts `llamacpp`, `vllm`, `mtplx`
- [x] 4.2 Per-runner preset fallback keys: model `hf` (llamacpp, vllm) / `model` (mtplx); context `ctx-size` (llamacpp, vllm) / `context-window` (mtplx)
- [x] 4.3 Refuse a local model path only where the destination seeds the weights (the cloud); the node path carries the path as the model to load — unblocking llamacpp and vllm node wakes with local weights as a side effect
- [x] 4.4 Add `model-id`, `context-window`, `max-active-requests` to the owned preset keys, and the `mtplx: "max-active-requests"` case to `parallelPresetKey`
- [x] 4.5 Tests: an mtplx wake from a Spinloop with and without a preset (per-runner fallbacks read back), an mtplx wake with a local model path, an llamacpp wake with a local model path now succeeding, the cloud still refusing both mtplx and local paths with today's messages

## 5. Docs and example

- [x] 5.1 `docs/commands/serve.md` and `docs/spinloop-file.md`: mtplx as a servable engine, its flag mappings, `--download`, and the scheduling mode staying in presets
- [x] 5.2 `docs/commands/remote.md`: mtplx is not a cloud runner (Apple-Silicon-only, no machine image)
- [x] 5.3 `docs/openapi.yaml`: add `mtplx` to the runner examples (`omlx` is absent too — add it while here)
- [x] 5.4 `examples/mtplx/`: a README and a `Spinloop`, mirroring `examples/omlx/`

## 6. Verify

- [x] 6.1 `gofmt -w ./...`, `go vet ./...`, `go test ./... -cover` (total coverage stays at 80% or above)
- [x] 6.2 `openspec change validate add-mtplx-engine`
