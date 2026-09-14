## 1. The file-side surface

- [x] 1.1 `internal/orchestrator`: `LockStatus(itemsPath) (pid int, alive bool, err error)` — the lock's holder and liveness, read-only, an absent lock being no holder
- [x] 1.2 `internal/orchestrator`: the abort marker beside the file — `RequestAbort(itemsPath, id)`, `AbortsPending(itemsPath)`, marker directory `<file>.aborts/`, one file per id, `O_EXCL`
- [x] 1.3 `internal/orchestrator`: `LoadStateFile(itemsPath)` — the state read with no lock and no recovery, an absent file an empty record
- [x] 1.4 `internal/orchestrator`: factor the file-joined-record view out of `WorkList.List` into a `Join(items, records)` both use

## 2. The run's convergence

- [x] 2.1 `WorkList`: consume the abort markers on a pass — stop the flights the way `Abort` does, drop the records, save, take the markers up when the children have ended; report the ids taken up
- [x] 2.2 the pass that took up a marker does not admit those ids; the next pass does
- [x] 2.3 the pass drops a record whose id the file no longer carries and that is not in flight; an in-flight item's record stands until it ends
- [x] 2.4 tests: a marker stops a running item and the pass that aborts does not re-admit; a marker for an item not in flight is consumed alone; a removed item's record is dropped; an in-flight item the file let go keeps its record until it ends

## 3. The commands

- [x] 3.1 `cmd/spinloop/work.go`: the `work` group — `add`, `list` (alias `ls`), `abort <id>`, `remove <id>`, each with `--items` defaulting to `./work.yaml`
- [x] 3.2 `work add`: the field flags — id, instructions, dir, tag (repeatable), priority — the validation, the duplicate-id and ended-record refusals, a missing file created with the one item
- [x] 3.3 `work list`: every item with its record in file order, a dash where a value is absent, the state colour on a terminal and plain lines elsewhere
- [x] 3.4 `work abort`: the running check, the no-orchestrator refusal, the marker, the bounded wait for the marker to be taken up, the outcome or the timeout naming the item
- [x] 3.5 `work remove`: the running refusal naming the abort that goes first, the file and the state and the log removed, the bounded wait for the record to be dropped where the run is running
- [x] 3.6 tests: each command's accepts and refusals, `list`'s plain and terminal output, the alias, and a run-and-command integration — a marker abort observed through the state, a remove converged by the pass

## 4. Docs

- [ ] 4.1 `docs/commands/work.md`: the family's usage — the file it works, the flags, the refusals, the hand-off to a running orchestrator
- [ ] 4.2 `docs/README.md` and `docs/work-items.md`: the pointer to the family
