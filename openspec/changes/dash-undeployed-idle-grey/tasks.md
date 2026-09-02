## 1. The tier and its glyph (cmd/spinloop/dashboard_render.go)

- [x] 1.1 Add a `dashNotServing` tier to the `dashHealthTier` enum, and
  extend the doc comments on the type, `dashHealthTierFor`, and
  `dashHealthGlyph` to place it: a settled report of `idle` or `stopped`
  with no action in flight, between the unknown cases and healthy.
- [x] 1.2 In `dashHealthTierFor`, return `dashNotServing` when the node
  answered, no action is in flight, the outcome is not a failure, the state
  is not `crashed`/empty, and the state is `idle` or `stopped` — a case
  after the empty-state unknown case and immediately before `default`, so
  no existing tier's precedence changes.
- [x] 1.3 In `dashHealthGlyph`, render `dashNotServing` as the faded grey
  dot `\033[90m●\033[0m` — the shade `dashUnknown` already uses for its
  `?`, so the two greys differ by mark, not colour.

## 2. Tests (cmd/spinloop/fleet_dashboard_test.go)

- [x] 2.1 Update `TestDashTileStoppedByteStable` to expect
  `dashHealthGlyph(dashNotServing)` on the `idle` node's name line.
- [x] 2.2 In the `dashHealthTierFor` table test, move the `idle` row from
  `dashHealthy` to `dashNotServing`, and add rows: a settled `stopped`
  report is `dashNotServing`; an `idle` report with a start in flight is
  `dashAttention` (never grey while starting).
- [x] 2.3 Confirm the remaining table rows are untouched: running+ready and
  running-without-readiness stay `dashHealthy`, `crashed` and the failed
  outcomes stay `dashUnhealthy`, no-refresh and no-state stay
  `dashUnknown`, action-in-flight stays `dashAttention`, and the
  selection-independent glyph test still passes.

## 3. Verify

- [x] 3.1 `go test ./...` passes, `go vet ./...` is clean, `gofmt -l ./...`
  prints nothing, and `go test ./... -cover` keeps total coverage >= 80%.
- [x] 3.2 Check no other surface drew the green for these states:
  `fleet metrics`, `fleet status`, and the detail view render unchanged
  (they draw no glyph; the byte-stable tests pin their lines).
