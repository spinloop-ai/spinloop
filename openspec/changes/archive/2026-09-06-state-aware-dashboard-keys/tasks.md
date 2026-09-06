# Tasks: state-aware dashboard key hints

## 1. Key offers on the model

- [x] 1.1 Add `startOffered` and `stopOffered` predicates on `dashModel` in `cmd/spinloop/dashboard_model.go`, beside `keepOffered` and `canAbort`: start offered when the described node exists, has no action in flight, and the board's current read of it does not report it running; stop offered when it exists, has no action in flight, and the read reports it running. Verify with table tests covering: each state (`idle`, `running`, `stopped`, `crashed`, `undeployed`), a read that failed, no read yet, a nil node, and each in-flight verb (`start`, `stop`, `keep`).
- [x] 1.2 Rebuild `gridKeys` and `detailKeys` to join their entries from the per-key offers — `move`/`back` and the board-wide keys always, then `s start`, `k keep`, `a abort`, `x stop` in each screen's existing order — so a key is named only where its offer is true. Verify with tests asserting each footer's full string for a running node, a stopped node, an unknown-state node, a nil node, and a busy node, on both screens.

## 2. Fold the abort filter into the construction

- [x] 2.1 Remove `dashFooterHints` from `cmd/spinloop/dashboard_render.go` and pass `m.gridKeys()` to `footerLine` directly from the grid's `View`, the abort entry now coming from `canAbort` at construction. Verify `go build ./...` succeeds and the abort's key help tests (named in the spec's "The key help hides abort when nothing is abortable" scenario) still pass unchanged in meaning.

## 3. Key help tests

- [x] 3.1 Extend the key help tests in `cmd/spinloop/fleet_dashboard_test.go` to the spec's scenarios — "The key help hides start and stop where they would do nothing" and "The key help offers start when the node's state is unknown" — driving the model through selection, reads, and actions rather than calling the predicates alone. Verify with `go test ./cmd/spinloop/`.
- [x] 3.2 Update the keep hint tests in `cmd/spinloop/dashboard_keep_test.go` that build the grid and detail key lines, so they hold the start and stop entries the offers now place beside the keep entry (for example, a kept remote environment that is running shows keep and stop, not keep and start). Verify with `go test ./cmd/spinloop/`.

## 4. Documentation

- [x] 4.1 Update the dashboard's key table in `docs/commands/fleet.md` so `s` and `x` carry the shown-only-where-they-drive-something wording `k` already has, and the prose around the detail view's keys agrees. Verify by reading the table against the spec's rule for each key.

## 5. Verification

- [x] 5.1 Run the full suite and checks: `go test ./... -cover` (total coverage stays at or above 80%), `go vet ./...`, `gofmt -l .` clean, `go build -o spinloop ./cmd/spinloop`. Verify all four pass.
