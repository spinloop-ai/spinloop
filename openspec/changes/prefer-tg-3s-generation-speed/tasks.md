# Tasks

## 1. Parse the three-second rate

- [x] 1.1 In `internal/metrics/speed.go`, add the `tg_3s` pattern tried before the unchanged `tg` pattern, sharing the validity filter, and rewrite the pattern/function doc comments to state the precedence. Verify with `go test ./internal/metrics/`.
- [x] 1.2 Rework `internal/metrics/speed_test.go` against the delta spec scenarios: `tg_3s` wins when both are present, a `tg_3s`-only line yields its figure, an unusable `tg_3s` (`0.00`, `-nan`) falls back to a usable `tg`, prompt-rate-only lines still yield nothing, and existing negatives (`n_tg`-style identifiers, missing `t/s`, zero/negative) stay pinned. Verify with `go test ./internal/metrics/ -run Speed -v`.

## 2. Update the consumers' expectations and notes

- [x] 2.1 Update `internal/daemon/speed_test.go`: the end-to-end fake prints both figures, so the expected reading becomes the `tg_3s` figure (5.29, not 3.87), and the "instantaneous preferred" comment reverses. Verify with `go test ./internal/daemon/ -run Speed`.
- [x] 2.2 Update the `Speed` field comments in `internal/metrics/metrics.go` that name the `tg` figure as what is read. Verify by inspection alongside the green `internal/metrics` suite.

## 3. Documentation

- [x] 3.1 Update the generation-speed paragraph in `docs/maintainer/internals.md` to describe the two-pattern precedence (`tg_3s` where usable, else `tg`) instead of the `tg`-only rule. Verify the paragraph reads true against the shipped parser.

## 4. Integration checks

- [x] 4.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./... -cover`; verify formatting and vet are clean, the suite is green, and total coverage stays >= 80%.
