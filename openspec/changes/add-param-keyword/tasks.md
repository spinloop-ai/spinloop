# Tasks: PARAM keyword

## 1. Grammar — `internal/spinloop`

- [x] 1.1 Add the `Param{Key, Value}` type and a `Params []Param` field to
  `Selection` (file order, like `Env`)
- [x] 1.2 Parse `PARAM`: repeatable, value is the rest of the line split at
  the first `=` (key a single non-empty token, value may be empty or hold
  spaces); special-case it before the single-value field-count check, as
  `ENV` is; parse errors name the line
- [x] 1.3 Add `PARAM` to the unknown-keyword error's accepted list
- [x] 1.4 Render `PARAM` lines in `Format` (after `PRESET`, before `ENV`)
- [x] 1.5 Tests: parse with trailing comment, repetition, repeated key,
  malformed values (no `=`, empty key, whitespace in key), unknown-keyword
  list, `Format` round-trip
- [x] 1.6 Accept a bare `PARAM key` — the ` = value` left off — as the bare
  form of `PARAM key =` (key is the rest of the line, value empty); the
  empty-key and whitespace-in-key errors apply to both forms. `Format` renders
  an empty-valued PARAM bare

## 2. Engine command — `internal/preset`, `cmd/spinloop/serve.go`

- [x] 2.1 Extract `Dialect.Merge(layers ...[]Param) []Param` from
  `Dialect.Flags` (identical later-wins merge); `Flags` renders the merge
- [x] 2.2 Add `modelSourceFlags(runner) []string` (llamacpp `hf-repo`/
  `model`/`hf-file`, mtplx `model`, vllm none) and a `modelFlags` field on
  `serveEngine` initialised from it; add `paramModel(engine, params)` — the
  last PARAM naming the model in the engine's dialect
- [x] 2.3 `buildServeArgv`: convert `sel.Params` to `[]preset.Param` once;
  preset path passes it as an override layer to `CommandIn` ahead of the
  instruction params; preset-less path renders
  `dialect.Merge(selParams, params)`
- [x] 2.4 Widen the `needsModel` check: `MODEL` or a PARAM naming the model
  (engines with model flags only); the error names the three ways
- [x] 2.5 Tests: PARAM renders per dialect (aliases, booleans, passthrough);
  PARAM beats preset; instructions beat PARAM; PARAM model for llamacpp and
  mtplx; vllm still needs `MODEL`; PARAM `ctx-size` unscaled by `PARALLEL`;
  `PARALLEL` beats PARAM's own value; preset-less PARAM-only command;
  `--dry-run` output
- [x] 2.6 Mention `PARAM` in `serve`'s help where `PRESET` is named

## 3. Cloud deploy — `cmd/spinloop/cloud.go`

- [x] 3.1 `deployConfig`: convert `sel.Params` to `[]preset.Param`; extend the
  model, context and parallel fallbacks with the PARAM layer (after the
  preset's); the model fallback reads the runner's model-source keys
- [x] 3.2 Append `dropOwned(owns, paramLayer)` as a ServeArgs layer after the
  section layer; pass the PARAM layer to `companionsFrom`
- [x] 3.3 Update the "nothing to deploy"/"no context size" errors to name a
  PARAM as an alternative
- [x] 3.4 Tests: deploy with PARAM only (model/context/parallel from PARAM,
  remaining PARAMs in ServeArgs minus owned flags); PARAM beats preset in the
  derived config; owned PARAM flags dropped for cloud, kept for node wake;
  node wake derives the same way; local-path PARAM model still refused by the
  cloud

## 4. Examples

- [x] 4.1 `examples/llamacpp/qwen3.8-27b-mtp/`: `Spinloop` (the issue's
  Q5_K_XL + `draft-mtp` configuration, PARAM-based, no preset file) and a
  `README.md` following the other llamacpp examples' shape
- [x] 4.2 `examples/vllm/qwen3.8-27b/`: `Spinloop` (first vLLM example:
  `PROVIDER vllm`, `MODEL`, `ALIAS`, `CONTEXT`, PARAMs with value-taking
  flags) and a `README.md`
- [x] 4.3 `examples/llamacpp/qwen3.8-27b-mtp/`: `PARAM jinja` written in the
  bare form, shown in the README

## 5. Docs

- [x] 5.1 `docs/spinloop-file.md`: `PARAM` in the keyword table, its syntax
  and repetition rules, the precedence rule (instructions > PARAM > preset),
  and the model-sourcing note
- [x] 5.2 Serve docs/help: `PARAM` alongside `PRESET` wherever the preset is
  described as the way to pass engine options
- [x] 5.3 The bare PARAM form (`PARAM jinja`) in `spinloop-file.md` and the
  serve docs' PARAM section

## 6. Verify

- [x] 6.1 `gofmt -l`, `go vet ./...`, `go test ./...` green
- [x] 6.2 Coverage `go test ./... -cover` at or above 80% overall
