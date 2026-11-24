## 1. Preset dialect

- [x] 1.1 Add `preset.Unsloth` with the aliases and boolean flags listed in design.md
- [x] 1.2 Unit-test alias canonicalisation and falsy-boolean dropping in the Unsloth dialect

## 2. Engine table

- [x] 2.1 Add a `defaults` field to `serveEngine` and merge it as the lowest layer in the preset, preset-less and deploy-config command paths
- [x] 2.2 Add `resolveUnslothBinary` (override, `PATH`, `$HOME/.unsloth/studio/unsloth_studio/bin/unsloth`)
- [x] 2.3 Add `unslothServeParams` (MODEL → `--model`, CONTEXT → `--context-length`, PARALLEL → `--parallel` unscaled, BASEURL → `--host`/`--port`, no ALIAS flag)
- [x] 2.4 Add the `unsloth` entry to `engines` (subcommand `run`, `needsModel`, `modelFlags`, defaults `--api-only`/`--disable-tools`, install hint, default base URL `http://127.0.0.1:8888`, loopback default bind, no key-file flag, no metrics)
- [x] 2.5 Add the `unsloth` case to `modelSourceFlags`, so a PARAM naming the model stands in for MODEL (the deploy-config preset-key helpers stay unchanged: wake and cloud deploy refuse `unsloth`)

## 3. Catalogue

- [x] 3.1 Add the `unsloth` provider to `internal/catalog/providers.yaml`
- [x] 3.2 Test the opencode and Pi blocks: keyless at localhost, `UNSLOTH_API_KEY` reference when set or remote

## 4. Tests

- [x] 4.1 Serve tests: dry-run command for MODEL, ALIAS+CONTEXT, PARALLEL, BASEURL, a PARAM naming the model, `disable-tools = false`, preset with `np` overridden by PARALLEL
- [x] 4.2 Binary lookup test: install-location fallback when not on the `PATH`
- [x] 4.3 Daemon tests: deploy-config argv carries the defaults; a keyed start is refused; endpoint defaults to port 8888 loopback
- [x] 4.4 Confirm other engines' commands are unchanged (existing suite passes); coverage stays ≥ 80%

## 5. Docs and example

- [x] 5.1 `docs/commands/serve.md`: engine table row, parallelism row, an "Unsloth Studio" section (flags, key, ALIAS, port moves when busy)
- [x] 5.2 `docs/guides/local-serving.md`, `docs/spinloop-file.md`, `docs/env-vars.md` (`UNSLOTH_API_KEY`, `UNSLOTH_BASE_URL`), `docs/index.md`, `README.md`
- [x] 5.3 `docs/openapi.yaml` runner examples
- [x] 5.4 `examples/unsloth/` guide with a Spinloop and README

## 6. Verify

- [x] 6.1 `go vet ./...`, `gofmt`, `go test ./...`
- [x] 6.2 `spinloop serve --dry-run` against the example on a machine with Studio installed, then a real run answering `/v1/chat/completions` with the ALIAS as the request model
