## Why

`spinloop work list` joined its columns with raw tabs, so an id longer than
one terminal tab stop threw every column after it out of line, and the table
carried no heading, so a reader had to already know the column order to make
sense of it. Separately, the orchestrator's work list API now answers callers
over the network — the CLI, and clients such as a GitHub action — and a call
it refused or a fault it hit left no trace in the run's own log; the operator
only ever saw what the caller saw.

## What Changes

- `work list` on a terminal renders as a table: each column as wide as its
  widest value (the heading included), opening with a heading row naming the
  columns (`ID  STATE  NODE  STARTED  ENDED`). Piped or redirected output is
  unchanged: tab-separated, one line per item, no heading.
- The orchestrator logs every work list API call once it is answered — the
  method, the path, the status, and how long it took — graded by outcome: a
  refusal a warning, a fault an error, an ordinary answer debug.

## Capabilities

### Modified Capabilities

- `work-commands`: the listing requirement gains the heading row and the
  column alignment for a terminal.
- `fleet-orchestrator`: the work list API's serving requirement gains that
  every call is logged with its outcome.

## Impact

- `cmd/spinloop/work.go`, `cmd/spinloop/work_test.go` — the table
- `internal/orchestrator/api.go`, `internal/orchestrator/api_test.go` — the
  call log
- `docs/commands/work.md` — the listing section's example and description
