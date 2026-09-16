## 1. The work list table

- [x] 1.1 Compute each column's width from the list — the heading included —
  and render `work list` on a terminal as a table opening with a heading row
  (`ID  STATE  NODE  STARTED  ENDED`), the state in its colour; leave the
  off-terminal output tab-separated and unpadded — verify with
  `go test ./cmd/spinloop/ -run TestWorkListTable -v` passing
- [x] 1.2 Cover a long id that used to throw a fixed tab stop out of line,
  checking every row's columns still start at the same place — verify with
  `TestWorkListTable_LongIDsStayAligned` passing

## 2. The work list API's call log

- [x] 2.1 Log every call the work list API answers, once the response is
  complete: the method, the path, the status, and the duration, graded by
  outcome (a refusal a warning, a fault an error, an ordinary answer debug) —
  verify with `go test ./internal/orchestrator/ -run TestAPI_EveryCallIsLoggedWithItsOutcome -v`
  passing
- [x] 2.2 Drop the two call-specific log lines the path already names (remove,
  abort) now the call log covers them; keep the add log, since the item's id
  is not otherwise in the URL — verify by reading `internal/orchestrator/api.go`
  against the `fleet-orchestrator` spec's new scenario

## 3. Documentation

- [x] 3.1 Rewrite the "Listing the work" section of `docs/commands/work.md` to
  describe the heading row and the column alignment on a terminal, alongside
  the existing off-terminal, tab-separated example — verify by reading it
  against the `work-commands` spec's new scenario

## 4. Tests and the suite

- [x] 4.1 Run the full suite with the race detector — verify with
  `go test ./... -race -cover` passing, no regressions, coverage holding at or
  above 80% in `cmd/spinloop` and `internal/orchestrator`
