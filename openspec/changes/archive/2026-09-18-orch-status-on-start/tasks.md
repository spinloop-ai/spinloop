## 1. Startup output

- [x] 1.1 In `runOrchestratorCommand` (`cmd/spinloop/orchestrator.go`), after
      `wl` (the `*orchestrator.WorkList`) is built and before the startup
      banner's `fmt.Printf` calls, keep printing the banner as it stands
      today.
- [x] 1.2 After the banner, print the work list from `wl.List()` using the
      same rendering `spinloop work list` uses (`workListTable` on a
      terminal, one `workListLine` per item otherwise), gated on
      `term.IsTerminal(int(os.Stdout.Fd()))` the way `workListCmd` gates it.
      Verify by running `go build ./...`.
- [x] 1.3 Update the command's `Long` help text to mention the startup work
      list. Verify with `spinloop orchestrator --help` showing the new
      sentence.

## 2. Tests

- [x] 2.1 Extend `TestCmdOrchestrator_LoopbackServesTheWorkListWithoutAToken`
      (or add a sibling test) in `cmd/spinloop/orchestrator_test.go` to
      assert the captured stdout contains the item's id and its state
      (`backlog`) after the banner. Verify with
      `go test ./cmd/spinloop/... -run TestCmdOrchestrator -v`.
- [x] 2.2 Add a test that seeds the state file beside the items file with a
      `done` record before starting the command, and asserts the startup
      output shows that item `done`, not `backlog` — covering the restart
      scenario. Verify with the same `go test` command.
- [x] 2.3 Run `go test ./... -cover` and confirm coverage has not dropped
      below the project's 80% floor.

## 3. Wrap-up

- [x] 3.1 Run `gofmt -l .` and confirm it reports no files.
