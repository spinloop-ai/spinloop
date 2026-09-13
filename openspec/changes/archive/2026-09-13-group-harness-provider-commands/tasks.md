# Tasks: group harness and provider commands

## 1. Move the commands in the tree

- [x] 1.1 Stop registering `add`, `remove`, `list`, `show`, `apply`, `unapply`, `export`, `init-providers` at the top level in `newRootCmd` (cmd/spinloop/commands.go:57): make `harness` a plain group (bare form via `groupFallback`) holding `addCmd`, `removeCmd`, `applyCmd`, `unapplyCmd`, `showCmd`, `exportCmd`, a new `openCmd` (the launch), and a new `configCmd` (report/store the default harness); and add a `providerCmd` group (mirroring `fleetCmd`) holding `listCmd` and `initProviderCmd` (the old `init-providers`, renamed `init` under the group). Verify: `go build ./...` passes, and against a clean `HOME` the help for `spinloop harness add`, `spinloop harness remove`, `spinloop harness apply`, `spinloop harness unapply`, `spinloop harness show`, `spinloop harness export`, `spinloop provider list`, and `spinloop provider init` matches the old top-level help.
- [x] 1.2 Move the launch body from the bare `harness` command into the new `openCmd` (`DisableFlagParsing`, `cobra.ArbitraryArgs`, the hand-rolled flag set, and the leading-Spinloop logic carried over; `ValidArgsFunction` renamed `launchSlot`), and move `--get`/`--set` into `configCmd` (`--get` the default, `--set <name>` stores). Rewrite `harness`'s `Short`/`Long` to describe the plain group and give `provider` a `Short`, all lowercase imperative phrases with no trailing full stop per `cli-ux`; update the root's long description to the new spellings. Verify: `go build ./...` passes; against a clean `HOME`, bare `spinloop harness` shows group help and launches nothing, `spinloop harness open` launches, and `spinloop harness config` / `--set` report and store.
- [x] 1.3 Update `harness`'s `Short`/`Long` and the root's long description to the new spellings; no old top-level spelling remains in any help output. Verify: `spinloop --help`, `spinloop harness --help`, and `spinloop provider --help` read as the other commands do.

## 2. Old spellings name their new home

- [x] 2.1 Give `newRootCmd` a custom `Args` validator (plus a help-only `RunE`, which the validator needs to be reached) mapping the eight removed top-level names to an error that names the new home (lowercase phrase, no trailing full stop, giving the new spelling — `spinloop init-providers` gives `spinloop provider init`), with every other word reproducing Cobra's unknown-command error, suggestions included. Verify: each of `spinloop add`, `spinloop remove`, `spinloop apply`, `spinloop unapply`, `spinloop show`, `spinloop export`, `spinloop list`, `spinloop init-providers` exits non-zero with an error giving its `spinloop harness …` / `spinloop provider …` replacement, and `spinloop bogus` still gets Cobra's unknown-command error.
- [x] 2.2 Add root-level tests asserting, for each of the eight old spellings, the error text (naming the new home) and the non-zero exit, plus the unchanged error for a genuinely unknown word. Verify: the tests pass under `go test ./cmd/spinloop/`.

## 3. Dispatch tests

- [x] 3.1 Add root-level tests that `spinloop harness <sub>` reaches the subcommand (not a launch) for all six, that a bare `spinloop harness` shows the group's help and launches nothing, that `spinloop harness <unknown-word>` gets the unknown-subcommand error, and that `spinloop harness open <args>` launches with arguments forwarded. Verify: the tests pass.
- [x] 3.2 Add root-level tests that `spinloop harness config` (and `--get`) report the active harness and that `spinloop harness config --set <name>` stores it, and that a flag before the subcommand word survives (`spinloop harness -H pi add …` reaches the `add` subcommand with the Pi harness selected) and that bare `spinloop provider` prints the group's help. Verify: the tests pass.
- [x] 3.3 Rework the existing launch and get/set tests that drove a bare `spinloop harness` to drive `spinloop harness open` and `spinloop harness config` instead; drop the subcommand/alias collision tests (the collision rule no longer exists, since `open` has no subcommands). Verify: the tests pass.

## 4. Completion

- [x] 4.1 Remove the tree-walking completion guard test's `harness` exception (the `harness` first slot now completes subcommands only, like every other group), and extend the `__complete` tests: `spinloop provider <TAB>` offers `list` and `init` with no file paths; `spinloop harness <TAB>` offers its subcommands (including `open` and `config`) with no file paths; `spinloop harness open <TAB>` offers the Spinloop names and paths a launch accepts; `spinloop harness config --set <TAB>` offers the harness names; `spinloop harness add -p <TAB>` offers the catalogue's providers. Verify: the completion tests pass.

## 5. Docs and examples

- [x] 5.1 Fold `docs/commands/add.md`, `remove.md`, `apply.md`, `unapply.md`, `show.md`, `export.md` into `docs/commands/harness.md` as subcommand sections (the way `remote.md` documents its subcommands), move `list.md` and `init-providers.md` into a new `docs/commands/provider.md`, and update every old spelling in `docs/` (including `getting-started.md`, `spinloop-file.md`, `env-vars.md`, `README.md`, `development.md`) and the repository `README.md`. Verify: `rg "spinloop (add|remove|list|show|apply|unapply|export|init-providers)\b" docs/ README.md` returns nothing.
- [x] 5.2 Update the commands in the `examples/*/README.md` files to the new spellings. Verify: `rg "spinloop (add|remove|list|show|apply|unapply|export|init-providers)\b" examples/` returns nothing.
- [x] 5.3 Rework `docs/commands/harness.md` (and the `docs/README.md` harness row, and any doc that says a bare `harness` launches) so the launch is documented under `spinloop harness open`, the get/set under `spinloop harness config`, and the bare `harness` as the group. Verify: `rg "spinloop harness\b(?! (open|add|remove|apply|unapply|show|export|config))" docs/ README.md` returns nothing, and no doc describes a bare `harness` as launching.

## 6. Full suite

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./... -cover`. Verify: no formatting or vet findings, all tests pass, and total coverage stays at or above 80% per AGENTS.md.

## 7. Spec sync (when archiving)

- [x] 7.1 At archive time, alongside applying the deltas, update the Purpose lines of the main specs that still name the old spellings (`harness-management`, `provider-catalog`, `model-discovery`, `spinloop-files`, `provider-selection`) to the new ones. Verify: after the sync, `rg "spinloop (add|remove|list|show|apply|unapply|export|init-providers)\b" openspec/specs/` returns nothing.
