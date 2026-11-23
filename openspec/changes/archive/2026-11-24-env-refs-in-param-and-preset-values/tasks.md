# Tasks

## 1. The expansion leaf

- [x] 1.1 Create `internal/envref` with `Expand(value string) (string, error)`
  implementing `$NAME`, `${NAME}`, `${NAME:-default}` and `$$`, literal `$`
  when not starting a defined form, and errors for unterminated `${`, invalid
  or empty names, and any other `${...}` form.
- [x] 1.2 Unit-test envref: each form, unset/empty variants, defaults (empty
  default included), `$$` and stray `$` literals, every error case, values
  with no references untouched.

## 2. Presets resolve at read

- [x] 2.1 `internal/preset.Parse`: expand each entry's value with
  `envref.Expand`, wrapping failures as `line %d: …`; keys untouched.
- [x] 2.2 Tests in `internal/preset`: a `[*]` value and a section value
  resolving from `t.Setenv`, a default standing when unset, a malformed
  reference failing with its line, key-containing-`${}` left literal.

## 3. PARAMs resolve at the choke point

- [x] 3.1 `cmd/spinloop`: make `selectionParams` return
  `([]preset.Param, error)`, expanding values with `envref.Expand` and naming
  the PARAM key on failure; update `buildServeArgv` and `deployConfig` to
  propagate.
- [x] 3.2 Tests: `buildServeArgv` renders a resolved PARAM over a preset's
  value and in `--dry-run` output; `deployConfig` derives model/window/args
  from resolved PARAMs; failures name the key from both paths.

## 4. End-to-end and guards

- [x] 4.1 Serve-path test: preset + PARAM + `ENV` interaction on a supervised
  path (cloud seed/daemon where env is applied) or via `t.Setenv`; resolved
  command printed, raw never shown.
- [x] 4.2 Guard tests: `Parse`/`Format` keep raw text (a Spinloop with a
  reference round-trips untouched); `apply`-path unaffected.
- [x] 4.3 `go vet ./...`, `gofmt`, full suite green with coverage >= 80%.

## 5. Docs

- [x] 5.1 Document the syntax on the Spinloop-file and preset docs pages
  (values only, `$$` escape, failure names the line/key), with a small
  example each.
