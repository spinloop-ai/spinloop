## Why

An item's kept agent output can only be read today by reaching the
orchestrator's own filesystem directly — `work list`/`add`/`abort`/`remove`
are already clients of the work list API (#228), but there is no client for
`GET /v1/items/{id}/log`. That defeats the point for a remote client — a
GitHub Action, a CI job — that only has network access to the API. (#231)

## What Changes

- `spinloop work logs <id>` reads an item's kept output through the work
  list API's existing `GET /v1/items/{id}/log`, the same `--url`/token
  shape as the rest of the `work` family.
- `-f`/`--follow` polls the API for new output and prints it as it arrives,
  the way `tail -f` does, reusing the `followUntilInterrupted` helper
  `fleet logs -f`/`remote logs -f` already share. It keeps polling through
  `backlog` and `running` — an item named before or just as it starts still
  gets followed — and stops once the item's state is `done` or `failed`, or
  on the operator's interrupt.
- An item with no output yet is printed as empty, not a fault, matching the
  API's own `log: null` answer.
- An id the run does not carry is refused, naming it, the way the other
  subcommands already are.

## Capabilities

### Modified Capabilities

- `work-commands`: the top-level `work` group's subcommand list gains
  `logs`; a new requirement covers reading and following an item's log
  through the work list API.

## Impact

- `cmd/spinloop/work.go`: `workLogsCmd`, reusing `workTarget`/`workRequest`
  the other subcommands already use
- `cmd/spinloop/follow.go`: `followUntilInterrupted` gains a second, third
  and fourth caller
- `openspec/specs/work-commands/spec.md`: the subcommand list and a new
  requirement
