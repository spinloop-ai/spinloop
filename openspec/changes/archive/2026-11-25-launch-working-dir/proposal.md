# Proposal

## Why

`spinloop code` and `spinloop harness open` launch the harness in the
directory spinloop was run from. A coding agent works on the project in its
working directory, so launching it for another project means changing
directory first. Applying a Spinloop by its path or alias already works from
anywhere, so the harness's directory is the only part of a launch that cannot
be chosen.

## What Changes

- `spinloop code` and `spinloop harness open` gain `-w`/`--working-dir <dir>`:
  the directory the launched harness runs in.
- A relative path is resolved against the directory spinloop was run from.
  A path that does not exist or is not a directory is an error, reported
  before anything is applied, so the harness config is left as it was.
- spinloop's own lookups are unchanged: `./Spinloop` for a bare `-O`,
  `./fleet.yaml`, and the `.env` beside the Spinloop are still found from the
  directory spinloop was run from.
- Tab completion offers directories, and only directories, for the flag's
  value, both as `-w <TAB>` and as `--working-dir=<TAB>`. The flag's value is
  consumed, so the Spinloop slot after it still completes.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `harness-management`: a new requirement for choosing the launched harness's
  working directory.
- `shell-completion`: completion covers directory values for the
  working-directory flag.

## Impact

- `cmd/spinloop/code.go`: the flag, its validation, and passing it to the
  launch. PR 286 edits a neighbouring flag line here.
- `cmd/spinloop/commands.go`: `launchAgent` sets the harness's directory.
- `cmd/spinloop/complete.go`: `launchValueFlags` and `launchSlot`.
- Docs: `docs/commands/harness.md` (`harness open`) and
  `docs/commands/code.md`.
