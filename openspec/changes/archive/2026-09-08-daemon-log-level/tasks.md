## 1. Core change

- [x] 1.1 In `internal/daemon/logging.go`, change `levelForStatus`'s success branch from `slog.LevelInfo` to `slog.LevelDebug`, and update its comment (which says raising the level to `warn` silences polling — now the default already does) and `summarize`'s doc if it describes the old grading. Verify `go build ./...` and `go vet ./internal/daemon` pass.

## 2. Daemon package tests

- [x] 2.1 `TestRequestSummaryFields` (`internal/daemon/logging_test.go`): build the daemon at `slog.LevelDebug` and assert the served request's summary carries `DEBUG` severity, not `INFO`. Verify `go test ./internal/daemon/ -run TestRequestSummaryFields -v` passes.
- [x] 2.2 `TestRequestSummaryGradesBySeverity`: expect the 200 summary at `DEBUG` (401/404/400 stay `WARN`). Verify `go test ./internal/daemon/ -run TestRequestSummaryGradesBySeverity -v` passes.
- [x] 2.3 `TestSummariesAreSilencedAtWarnButFailuresAreNot`: extend it to also cover the default level — at `slog.LevelInfo` the successful polls emit no summary and the 401 does. Verify `go test ./internal/daemon/ -run TestSummariesAreSilenced -v` passes.

## 3. CLI tests

- [x] 3.1 `TestCmdDaemon_SummarisesRequestsByDefault` (`cmd/spinloop/logging_test.go`): rework it to assert the new default — a successful status request leaves no `api request` line in the log, while a rejected one is recorded at `WARN` (renaming the test to say so). Verify `go test ./cmd/spinloop/ -run 'TestCmdDaemon_Summarises|TestCmdDaemon_LogLevelSilences' -v` passes.
- [x] 3.2 `TestCmdDaemon_LogLevelFlagBeatsEnvironment`: the flag now needs `--log-level debug` (not `info`) to make a successful request's summary appear over `SPINLOOP_LOG_LEVEL=error`. Verify `go test ./cmd/spinloop/ -run TestCmdDaemon_LogLevelFlagBeatsEnvironment -v` passes.

## 4. Docs

- [x] 4.1 `docs/commands/serve.md` "What gets logged": the level table — `debug` gains every successful request summary, `info` (default) drops "every request" (it keeps starts, stops, clean exits plus rejections and failures) — and the fleet-polling note, which now says polling is quiet at the default level and `--log-level debug` is how to see it.
- [x] 4.2 `docs/http-api.md`: the status→level table (2xx/3xx → `debug`) and the "a node that a fleet polls will log a line per poll per client at the default level" sentence.
- [x] 4.3 Sweep `docs/README.md`, `docs/env-vars.md` and the rest of `docs/` for any sentence asserting request summaries appear at the default level, and fix what the sweep finds. Verify `grep -rn "at the default" docs/` returns nothing that contradicts the new behaviour.

## 5. Full verification

- [x] 5.1 `go test ./... -cover` passes with total coverage >= 80%, `go vet ./...` is clean, and `gofmt -l .` names no files.
