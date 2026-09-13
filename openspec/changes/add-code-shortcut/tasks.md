## 1. Shared launch builder

- [ ] 1.1 Extract the `openCmd()` body — the `DisableFlagParsing` + `splitHarnessArgs` `RunE`, the full flag set, and the `launchSlot` completion — into `openLaunchCmd(use, short, long string) *cobra.Command` in a new `cmd/spinloop/code.go`, and reduce `openCmd()` to a one-line call to it. Verify: `go build ./...` passes and the existing harness/open tests still pass unchanged (e.g. `go test ./cmd/spinloop/ -run 'TestComplete_HarnessOpenOffersSpinloop|TestRoot'`).

## 2. The code command and its registration

- [ ] 2.1 Add `codeCmd()` (returns `openLaunchCmd("code", …)` with `Short`/`Long` stating it is a shortcut for `harness open`) and a `cmdCode(args)` test seam in `code.go`. Verify: `go build ./...` passes.
- [ ] 2.2 Register `codeCmd()` in the root's `AddCommand` list in `commands.go`, next to `upCmd()`. Verify: the built binary's `spinloop code --help` prints the code help and its launch flags, and `spinloop harness open --help` still prints the open help.

## 3. Tests

- [ ] 3.1 Add a root-dispatch test that `spinloop code` reaches the launch (a bare `code` with a stub harness launches it), mirroring the `up`/harness dispatch tests. Verify: the new test passes.
- [ ] 3.2 Add parity tests that `code` behaves as `harness open` for the key cases: a bare launch, a leading alias, a valueless `-O`, `--env`, and trailing-argument forwarding. Verify: the new tests pass.
- [ ] 3.3 Add `TestComplete_CodeOffersSpinloop` asserting `spinloop code <TAB>` offers registered alias names and paths, and confirm `code` appears in the command-name completion and the tree-walking guard. Verify: `go test ./cmd/spinloop/ -run TestComplete` passes.
- [ ] 3.4 Run the full suite with coverage. Verify: `go test ./... -cover` passes and total coverage stays >= 80%.

## 4. Docs and final checks

- [ ] 4.1 Add a `docs/commands/code.md` reference page (the one-word shortcut, its flags, and that it is `harness open`), parallel to `docs/commands/up.md`, and link it from the command index and any prose that names the launch. Verify: the command index lists `code` and the page is consistent with the others.
- [ ] 4.2 Run the repository's checks. Verify: `gofmt -l .` is empty and `go vet ./...` passes.
