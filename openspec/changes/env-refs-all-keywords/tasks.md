# Tasks

## 1. The scalar resolve choke point

- [ ] 1.1 Add `resolveSelection(sel) (Selection, error)` in cmd/spinloop
      (new `resolve_refs.go`) resolving the eight scalar values
      (`Provider`, `Model`, `Alias`, `Context`, `Output`, `Parallel`,
      `BaseURL`, `Preset`) via `envref.Expand`, each error wrapped by
      keyword (`MODEL: …`); it leaves `Params` and `Env` untouched. Unit
      tests in `resolve_refs_test.go` cover a resolved value, a keyword
      error name, and Params/Env staying raw in the returned copy.
- [ ] 1.2 Teach `applySpinloopEnv` to `envref.Expand` each ENV value before
      `os.Setenv` (after the `.env` pass), error wrapped `ENV <KEY>: …`,
      keys never expanded. Verified by a `cloud_env_test.go`/`env_refs_test.go`
      case: `ENV SLOT=${SLOT_BASE:-1}` exports `SLOT=1`, an `ENV ${FOO}=1`
      sets a variable literally named `${FOO}`, and a malformed value names
      the key.

## 2. Wire the consumers

- [ ] 2.1 Call `resolveSelection` in `runServe` right after `readSpinloop`
      (before `engineFor`), so `serve`'s MODEL/BASEURL/CONTEXT/PRESET/
      PROVIDER resolve. Verified by an `env_refs_test.go` `serve --dry-run`
      case: `MODEL org/model:$HF_TAG` prints `org/model:Q4_K_M`, and a
      malformed `MODEL …${HF_TAG` fails naming `MODEL`.
- [ ] 2.2 Call `resolveSelection` in `deriveDeployTarget` after
      `applySpinloopEnv` (covers `cloud deploy` and `fleet deploy`) and in
      the seed path, so the deploy config and the `org/model:quant` split
      read resolved text. Verified by a `deployConfigFor`-level test:
      `MODEL org/model:$HF_TAG` yields a resolved `ModelID`/`Quant`, and a
      `PRESET ${PRESETS_DIR}/preset.ini` loads from the resolved path.
- [ ] 2.3 Call `resolveSelection` in the `apply`/`unapply` bodies and the
      code.go launch path, so `apply` writes and `unapply` removes the
      resolved value. Verified by an `apply_test.go`/`env_refs_test.go`
      case: `BASEURL http://${HOST:-localhost}:8080/v1` writes the resolved
      URL to the harness config, and apply+unapply round-trip removes it.
- [ ] 2.4 Call `resolveSelection` in `fleet start`/`fleet route`, the
      gateway per-node read, and `spinloop alias` (resolved `ALIAS` names
      the registration). Verified by a route test (routes by the resolved
      model), a gateway node-list test, and an `alias_test.go` case
      (`ALIAS ${NICK}` registers as `qwen`).

## 3. Docs and full-suite verification

- [ ] 3.1 Update the Spinloop file-format docs (under `docs/`) to state the
      substitution applies to every value (with the `$$` escape and the
      key/keyword exclusion); a docs reader sees the rule once, not per
      keyword.
- [ ] 3.2 Run `go test ./... -cover` and `go vet ./...`: the whole suite is
      green and coverage stays >= 80%.
- [ ] 3.3 `gofmt -w ./cmd ./internal`; verify `gofmt -l ./cmd ./internal`
      prints nothing.
