## Why

The orchestrator's backlog lives in a work items file beside the machine it
runs on, and the only ways to touch it today are to edit the file by hand or
to use the work list API of a run that listens on an address and a token the
operator may not have. An operator with a terminal next to the file — the
common case — has no command that adds an item, reads the backlog's state,
stops a running item, or removes an item the way the run would. The four
`spinloop work ...` commands close that gap: the items file and the state
beside it become workable from the shell, whether or not the orchestrator is
running.

## What Changes

- A new `spinloop work` command group with four subcommands, each working the
  work items file a shared `--items` flag names (default `./work.yaml`):
  - `work add` — an item's fields from flags, appended to the file with the
    validation the orchestrator applies; a duplicate id and an id the state
    has recorded ended are refused, naming the collision or the record
  - `work list` — every item in the file with its record from the state
    beside it: state, the node a running item runs on, when it started and
    ended; an item with no record is backlog, and no state file at all means
    everything is backlog; plain output a program can consume, `ls` as an
    alias
  - `work abort` — a running item's agent stopped and its record removed, so
    the item is back in the backlog; only a running item can be aborted, and
    with no orchestrator running there is nothing running
  - `work remove` — an item out of the file, its record out of the state, and
    its log gone; a running item is refused, naming it and the abort that
    goes first
- The run gains the convergence those external edits owe it: a record the
  items file no longer carries is dropped from the state, and an abort marker
  beside the file stops a running item the way the work list API's abort
  does — the agent stopped, the record gone, the item admitted again on the
  next pass.

## Capabilities

### New Capabilities
- `work-commands`: the `spinloop work` command group — add, list, abort and
  remove against the work items file and the state the orchestrator keeps
  beside it, working whether or not the orchestrator is running.

### Modified Capabilities
- `fleet-orchestrator`: the run's pass gains two obligations — it drops a
  record the items file no longer carries, and it takes up an abort marker
  beside the file, stopping the marked item's agent and putting the item back
  in the backlog.

## Impact

- `cmd/spinloop/work.go` — the `work` command group and its four
  subcommands, new
- `internal/orchestrator` — the lock's liveness check, the abort marker
  beside the file, a state read that takes no lock and performs no recovery,
  and the run's convergence: marker consumption and record pruning, in the
  pass
- Tests for the file-side operations, the run's convergence, and each
  command's accepts and refusals
- `docs/commands/work.md` — the family's usage; `docs/work-items.md` and
  `docs/README.md` gain a pointer

The GitHub action that works the items file from issue events is a separate
repo and a later change (#223); it calls these commands and is out of scope
here.
