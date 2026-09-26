# Add a kanban board for the work list

Closes #246.

## Why

The work list is only legible one item at a time: `work list` prints a
stateless table, and while it answers "what is there", it cannot answer "how
is the run doing" — how much sits waiting, how many nodes are busy, which
items ended badly — without the operator reading every row. The fleet has a
full-screen view of its own; the work list, the thing the orchestrator is
actually working through, has nothing to watch.

## What Changes

- New `spinloop work board` command: an interactive, full-screen kanban
  board of the orchestrator's work list. Four fixed columns — Backlog,
  Running, Done, Failed — with one card per item, re-read from the work list
  API on a cadence so the board moves as the run works.
- Each card shows its item's id, instructions (clipped), priority, node and
  timing; the selected item's full detail opens on enter — instructions in
  full, tags, the failure reason, and the item's kept log output tailed
  live, the way the fleet dashboard's detail pane tails node logs.
- From the board the operator acts through the same work list API the
  one-shot commands use: abort a running item, remove a non-running one
  (asking first), quit. Keys are offered only where they would do something.
- Adding an item from the board: `n` opens a form modal over the live
  board — id, instructions, working directory, tags, priority — sent to
  the API's own add path, which stays the only judge of an item's shape;
  a refusal lands in the form, worded as the API states it, to be
  corrected and resent. `work add` stays the scripted way in.
- The command follows the work family's conventions exactly: `--url` names
  the API, the token comes from `--api-token` / `--api-token-file` /
  `SPINLOOP_API_TOKEN`, and with no terminal to draw on it refuses, naming
  `spinloop work list` as the pipe alternative.
- No server-side change: everything the board shows and does is served by
  the existing work list API (`GET /v1/items`, add, abort, remove, log).

## Capabilities

### New Capabilities

- `work-board`: the full-screen kanban view of a running orchestrator's
  work list — columns and cards, selection and detail, the keys that act
  through the work list API including the form modal that adds an item,
  refresh and staleness, and the terminal gate.

### Modified Capabilities

- `work-commands`: the group requirement "The work commands as work list
  clients" enumerates the subcommands; `board` joins the family as another
  client carrying the same `--url` and token conventions.

## Impact

- New `cmd/spinloop/work_board.go` (command + program gate),
  `work_board_model.go` (model, polling, keys, the add form) and
  `work_board_render.go` (columns, cards, detail, the form), following
  the split the fleet dashboard uses.
- `cmd/spinloop/commands.go` / `complete.go`: the subcommand and its
  completion registration.
- Reuses: the `work` family's flag and token plumbing (`workAPIFlags`,
  `workTarget`, `workRequest`), `orchestrator.ItemView` as the wire type,
  `palette.go` colours under the cli-ux accent/state split, and the
  existing charmbracelet dependencies (bubbletea, lipgloss, x/ansi). One
  new direct dependency: `charmbracelet/bubbles`, used for its
  `textinput` and nothing else — and in one place after
  [#248](https://github.com/spinloop-ai/spinloop/issues/248) folds the
  dashboard's keep prompt onto the same pattern.
- No change to the orchestrator, its API, the items file, or the state.
