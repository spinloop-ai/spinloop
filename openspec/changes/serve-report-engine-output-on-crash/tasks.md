# Tasks

## 1. Report the captured log when the engine crashes

- [x] 1.1 Add `reportEngineLogTail(w io.Writer, logPath string)` to
  `cmd/spinloop/serve_view.go`: a header naming `logPath`, then the bounded
  tail via `daemon.ReadLog(logPath, daemon.TailLog, 40*bytesPerLineGuess)`
  verbatim, or a plain note when the log is empty, missing, or unreadable.
  Verify: unit tests in `cmd/spinloop/serve_view_test.go` covering a log with
  content (tail and path appear), a missing log, and an empty log.

- [x] 1.2 Call it from `runServeView` when `run.wait()` returns a non-nil
  error (the run ended in a crash), writing to `os.Stderr` before returning
  the error, so the stop-on-request and clean-exit paths print nothing extra.
  Verify: `go build ./...` and the existing serve suite
  (`go test ./cmd/spinloop/ -run Serve`) still pass.

## 2. Pin the rule end to end

- [x] 2.1 Add a test that runs `runServeView` against a stub engine that
  writes to stderr and exits non-zero, with `newServeProgram` driven with
  injected input/output, and assert the run's stderr carries the engine's own
  error line under a header naming the engine log's path, and that the run
  still exits with the engine's exit status. Verify: the test fails without
  1.2 and passes with it.

- [x] 2.2 Run `gofmt -l .`, `go vet ./...` and `go test ./... -cover` and
  bring the suite green with total coverage >= 80%.

## 3. Documentation

- [x] 3.1 Update the user documentation for `spinloop serve` (docs site,
  under `docs/`) where it describes the serve view or engine logs, so it says
  a failed engine's log tail is printed to stderr when the view closes.
  Verify: `grep` the docs for the new wording; `mkdocs` build is unaffected
  (no mkdocs.yml nav change needed unless a new page is added).
