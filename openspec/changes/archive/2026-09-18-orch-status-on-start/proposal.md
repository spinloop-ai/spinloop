## Why

`spinloop orchestrator` starts with a one-line banner naming the items file,
the gateway, and the backlog count, but nothing shows which items are in the
backlog, what state they carry, or what a restart recovered. An operator has
to run `spinloop work list` against the freshly-started API to see that —
one more step every time the command starts.

## What Changes

- On startup, after the existing banner, the orchestrator prints the work
  list itself — the same table a terminal gets from `spinloop work list`,
  or the same plain tab-separated lines a pipe gets, drawn from the run's
  view of the items before the first admission pass runs.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-orchestrator`: the orchestrator command's startup requirement gains
  a scenario — the startup output includes the work list, not just the
  banner naming the file, the gateway, and the count.

## Impact

- `cmd/spinloop/orchestrator.go`: `runOrchestratorCommand` prints the list
  after building the `WorkList`, reusing the rendering `cmd/spinloop/work.go`
  already has for `spinloop work list` (`workListTable` / `workListLine`,
  chosen by whether stdout is a terminal).
- No API, storage, or wire-format change — the output is additional text on
  the process's stdout at startup.
