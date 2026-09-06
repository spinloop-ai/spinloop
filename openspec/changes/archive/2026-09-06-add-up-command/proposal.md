# Proposal: the `up` command

## Why

Starting the engine for what is in the current directory is the most repeated
operation, and its target depends on the directory: a Spinloop file names a
local engine (`spinloop serve`), while a `fleet.yaml` names a fleet of
machines (`spinloop fleet start`). Two verbs for one gesture. `up` gives the
gesture one word and picks the target from the directory.

This change originally proposed `up` as an alias of `serve`; it has since been
redesigned as a dispatching command, because an alias has one fixed target and
`up`'s target depends on the working directory. This proposal describes the
current design.

## What Changes

- A new top-level command, `spinloop up`, taking positional arguments only.
  - In a directory holding a `fleet.yaml`, `up [node…]` starts the fleet's
    engines: the named nodes when given, every node when given none — the
    `--all` form of `fleet start`, which refuses to run bare.
  - Otherwise, `up [path]` is exactly `serve [path]`: the same Spinloop
    resolution (path, registered alias, `SPINLOOP_ALIAS`, `./Spinloop`), the
    same output, and the same errors — including `serve`'s "no Spinloop
    found" failure when nothing resolves.
  - A `fleet.yaml` in the directory wins over a Spinloop.
- `serve`, `fleet start`, and everything they call are unchanged: `up`
  dispatches to them, so the two cannot drift apart.
- No flags on `up`: it is the quick path, and a flag that means `--all` in one
  directory and `--dry-run` in another would be two commands wearing one name.

No breaking changes; `serve` and `fleet start` keep working exactly as before.

## Capabilities

### New Capabilities

- `up-command`: the `spinloop up` command — how it chooses between starting
  the fleet and starting the local engine, what it forwards, and how it fails.

### Modified Capabilities

(None. The earlier delta to `local-serving` is retracted: `serve` itself does
not change.)

## Impact

- `cmd/spinloop/up.go` (new) — the command and its dispatch, reusing the fleet
  resolution and start path and `serve`'s own body; registered in
  `cmd/spinloop/commands.go`.
- `cmd/spinloop/up_test.go` (new) — the dispatch rules.
- `docs/commands/up.md` (new) and a row in `docs/README.md`'s command table.
- Not touched: `serve`, `fleet start`, the alias registry, and the existing
  completion surface (the command tree picks `up` up automatically).
