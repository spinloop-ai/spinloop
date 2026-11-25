# Design

## Context

`openLaunchCmd` in `cmd/spinloop/code.go` builds both `code` and
`harness open`. Flag parsing is off: `splitHarnessArgs` parses spinloop's
flags out of the arguments with the command's own flag set and forwards the
rest. `launchAgent` runs the harness with `exec.Command` and no `Dir`, so the
harness inherits spinloop's working directory.

Completion for these commands goes through `launchSlot` in
`cmd/spinloop/complete.go`, which reads the unparsed words. `launchValueFlags`
lists the flags whose next word is a value, so the positional count skips it.

## Goals / Non-Goals

**Goals:**

- Choose the harness's working directory per launch.
- Complete directories for the flag's value.

**Non-Goals:**

- Moving spinloop's own lookups (`./Spinloop`, `./fleet.yaml`, `.env`) to
  the working directory. They keep working as they do now, so a launch's
  meaning does not change depending on where the agent will run.
- Passing the directory to the harness as an argument (opencode's positional
  project path, for example). Setting the process's working directory works
  for every harness the same way.

## Decisions

**Flag registered on the shared builder.** `fs.StringVarP(&workDir,
"working-dir", "w", …)` in `openLaunchCmd`, so `code` and `harness open`
behave the same. `splitHarnessArgs` already consumes any flag in the set
together with its value, so `-w` is never forwarded.

**Validated first.** Right after the argument split, and before
`harness.Resolve` and any apply, the path is made absolute with
`filepath.Abs` and checked with `os.Stat`. A missing path gives
`working directory <path> does not exist`, and a non-directory gives
`working directory <path> is not a directory`. The config is untouched in
both cases.

**Passed to `launchAgent` as `dir`.** `cmd.Dir = dir` when it is not empty.
This is one added parameter on a function with a single caller.

**Completion.** `--working-dir` and `-w` are added to `launchValueFlags`.
`launchSlot` returns `cobra.ShellCompDirectiveFilterDirs` both for a detached
`-w`/`--working-dir` right before the cursor and for the attached
`--working-dir=<partial>` form. `compRegister(c, "working-dir", compDirs)`
marks the flag, so `defaultFlagCompletions` does not register the no-values
completion over it. `compDirs` is a new one-line function that returns
`FilterDirs`.

## Risks / Trade-offs

- A harness flag spelled `-w` now has to come after the first forwarded
  argument, or after `--`. None of the supported harnesses' launch flags use
  `-w`, and this is the same rule every other spinloop flag on these commands
  follows.
- PR 286 edits the `--node` line in the same flag block, so the two may
  conflict textually. The resolution is to keep both lines.
