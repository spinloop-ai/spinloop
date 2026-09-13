## Context

`spinloop harness open` is built by `openCmd()` in `cmd/spinloop/commands.go`.
It is a `DisableFlagParsing` command: Cobra hands it every word on the line, and
its `RunE` runs `splitHarnessArgs` over them — recognising the launch's own flags
(`-O`/`--spinloop`, `--env`, `--fleet`, `--node`, `--prefer`, `--no-wake`,
`--wake-timeout`, `-H`, `--providers`) wherever they appear, consuming one leading
argument that names a Spinloop, and forwarding the rest to the harness
byte-for-byte. Its `ValidArgsFunction` is `launchSlot`. `upCmd()` in
`cmd/spinloop/up.go` is the existing top-level one-word command, registered in the
root's `AddCommand` list and given a `cmdUp` test seam that calls
`execCmd(upCmd(), args)`.

See proposal.md for why `code` is wanted. The whole design question is how to make
`code` identical to `harness open` without a second copy of the launch logic.

## Goals / Non-Goals

**Goals:**
- `spinloop code <args>` is behaviourally identical to
  `spinloop harness open <args>`: same flags, same Spinloop application, same
  launch, same forwarding, same output and errors.
- The two are built so they cannot drift — one implementation, two spellings.

**Non-Goals:**
- No change to `harness open`, the harness config subcommands, or any harness
  adapter.
- No new flags on `code` beyond what `harness open` already accepts.
- No project/pwd selection or other behaviour that would distinguish `code` from
  `harness open` — it is a pure alias.

## Decisions

**One shared launch builder, two commands.** Factor the body of `openCmd()` into a
single function `openLaunchCmd(use, short, long string) *cobra.Command` that builds
the command with its `Use`, the `DisableFlagParsing` + `splitHarnessArgs` `RunE`,
the full flag set, and the `launchSlot` completion. `openCmd()` returns
`openLaunchCmd("open", …)` and is added to the `harness` group as today; `codeCmd()`
returns `openLaunchCmd("code", …)` and is added to the root. They differ only in
`Use` and the `Short`/`Long` copy (which says `code` is a shortcut for
`harness open`).

- **Alternative considered:** make `code` a thin wrapper whose `RunE` calls
  `execCmd(openCmd(), args)`. Rejected: `openCmd()` hard-codes `Use: "open"`, so
  `code`'s help, usage line, and completion descriptions would say "open", and a
  wrapper still needs a flag set to be completed. The builder gives both commands
  their own correct identity from one implementation.
- **Placement:** `openLaunchCmd`, `codeCmd`, and the `cmdCode` seam live in a new
  `cmd/spinloop/code.go`, mirroring `up.go`; `openCmd()` becomes a one-line call to
  the builder. `commands.go` keeps registering `openCmd()` under `harness` and gains
  `codeCmd()` in the root's `AddCommand` list (next to `upCmd()`). `rootArgs` and
  `movedTopLevelCommands` are untouched — `code` is a new command, not a moved one.

## Risks / Trade-offs

- [`code` and `harness open` diverge if one is edited without the other] → Both
  are constructed by the single `openLaunchCmd` builder; there is no second flag
  set or `RunE` to edit independently.
- [A user expects `code` to do more than `harness open` (e.g. pick a project)] →
  The spec fixes `code` as a pure alias and the help text says so; a distinct
  behaviour would be a separate, later change.
- [One more top-level command widens the root surface] → Acceptable: `up` already
  set the precedent for a one-word top-level shortcut, and the completion guard
  walks the tree, so `code` is covered automatically.

## Migration Plan

None — the change is additive and breaks nothing. Deploy is the release; rollback
is reverting the commit.
