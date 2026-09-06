## 1. Renderer

- [x] 1.1 Rewrite `formatKeepDuration` in `cmd/spinloop/metrics_render.go` to
      render in hours and minutes only: floor the remainder to the minute,
      render a sub-minute remainder as `1m`, drop any zero unit (`2h`, `24m`,
      `1h 30m`), and carry no seconds component; update its doc comment to
      state the granularity. Verify by reading the function and confirming no
      `s` unit is produced on any branch
- [x] 1.2 Add cases to the `TestKeepDurationRendersRelatively` table
      (`cmd/spinloop/retain_render_test.go`): `2h 5m 30s` → `keep for 2h 5m`,
      `24m 30s` → `keep for 24m`, and `45s` → `keep for 1m`; verify with
      `go test ./cmd/spinloop/ -run TestKeepDurationRendersRelatively`

## 2. Verification

- [x] 2.1 Run `go test ./... -cover`, `go vet ./...`, and `gofmt -l .` and
      verify the suite is green with total coverage still >= 80% and no
      formatter output; the existing keep-figure assertions in
      `retain_render_test.go` and `dashboard_keep_test.go` already carry no
      seconds and must stand unchanged
