## 1. Shared follow cursor

- [x] 1.1 Add `internal/remote.FollowCursor` (dedup by event id over an
      overlap window) and `internal/remote.FollowOverlap`, extracted from
      `cmd/spinloop/remote_logs.go`'s inline follow state.
- [x] 1.2 Add unit tests for `FollowCursor`: start bound before/after an
      event is seen, dedup of already-seen ids, pruning outside the overlap
      window, and `Reset`.

## 2. Standalone command uses the shared cursor

- [x] 2.1 Rework `cmd/spinloop/remote_logs.go`'s `followLogsLoop` to use
      `remote.NewFollowCursor(remote.FollowOverlap)` instead of its own
      `printed`/`newest` maps, preserving existing behavior.
- [x] 2.2 Confirm the existing `TestFollow*` tests in `remote_logs_test.go`
      still pass unmodified (they exercise `followLogsLoop` as a black box).

## 3. Remote fleet node uses the shared cursor

- [x] 3.1 Give `remoteNode` its own `*remote.FollowCursor`, created in
      `NewRemoteNode`.
- [x] 3.2 Rework `remoteNode.Logs` to reset the cursor on `daemon.TailLog`,
      bound its query with the cursor's `Start()`, and run the result
      through `Advance()` before mapping it to a `daemon.LogsResponse`.
- [x] 3.3 Rework `logsFromRemote` to take the already-deduped fresh events
      and a `missing` flag (true only for a from-the-beginning read that
      found nothing), rather than a raw `LogResult` and an offset.
- [x] 3.4 Update `internal/fleet/remote_node_test.go`: `TestLogsFromRemote`
      for the new signature and missing-vs-quiet distinction, and a test
      that a follow-up poll bounds its CloudWatch query using
      `remote.FollowOverlap` behind the last event shown.

## 4. Verify

- [x] 4.1 `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- [x] 4.2 Confirm no other caller of `logsFromRemote` or `remoteNode.Logs`
      needs updating for the new signatures.
