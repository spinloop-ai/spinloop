## Why

`spinloop harness open` is the launch, but it sits behind the `harness` group, so
the most common action — "start my configured coding agent" — costs two words.
`spinloop up` already shows the pattern for a one-word top-level shortcut; `code`
does the same for the harness launch, so the primary action is as short as
`spinloop up`.

## What Changes

- Add a new top-level `spinloop code` command that is a shortcut for
  `spinloop harness open`. It accepts the same flags, applies a Spinloop the same
  way (a leading alias or path, `-O`/`--spinloop`, `--env`, `--fleet`, `-H`),
  launches the active harness, and forwards trailing arguments untouched — so
  `spinloop code …` behaves exactly as `spinloop harness open …`.
- `code` and `harness open` share one launch implementation so the two cannot
  drift.
- Tab completion offers the Spinloop slot on `code`, exactly as on
  `harness open` (registered alias names and paths).

`spinloop harness open` itself is unchanged — `code` is a new alias, not a
rename, so nothing is **BREAKING**. The shell-completion surface picks `code` up
automatically (it is derived from the command tree), so no requirement there
changes.

## Capabilities

### New Capabilities
- `code-command`: the top-level `spinloop code` shortcut — that it is a one-word
  alias for `spinloop harness open`, sharing its flags, Spinloop application,
  launch, and argument forwarding, and that its tab completion offers the
  Spinloop slot.

### Modified Capabilities
<!-- None. The shell-completion surface is derived from the command tree, so the
     new `code` command is covered without a requirement change. -->

## Impact

- `cmd/spinloop/commands.go` — register `code` at the root and factor `open`'s
  launch command into a shared builder used by both `harness open` and `code`.
- `cmd/spinloop/code.go` (new) — the `codeCmd` command and `cmdCode` test seam,
  alongside the `up.go` pattern.
- Tests — root command-tree dispatch, `code` launch parity with `harness open`,
  and `code`'s completion (the completion guard walks the tree, so the command
  name is covered automatically).
- Docs — a `code` command reference page, the command list, and any prose that
  names the launch.
- No change to `harness open`, the harness config commands, or any harness
  adapter.
