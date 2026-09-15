## Context

See proposal.md — Why. The implementation-relevant current state:

- `resolveFleetTarget` (`cmd/spinloop/target.go`) already turns `--env` /
  `--fleet` / the working directory into the `*fleet.Config` to act on, and
  already holds the rule that the two flags are exclusive. Eight commands call
  it. A top-level verb is a ninth caller, not a new mechanism.
- `renderFleetStatus` and the dashboard model (`dashboard_model.go`,
  `dashboard_render.go`) are already independent of the command that opens
  them: `fleetStatusCmd` is a flag block plus one fan-out call, and
  `fleetDashboardCmd` a flag block plus `dashModelFor` and `runDashProgram`.
- `runRemoteStatus` (`cmd/spinloop/remote.go`) calls `remote.Status` directly
  and renders a key-value block. It already shares `statusFact` with the fleet
  table — the facts converged, the layout did not. It also makes a second call
  for the version, applies a Spinloop's `ENV` before resolving, and renders the
  address and retention deadline: three things a fan-out does not do, which is
  why it stays until `metrics` and `logs` force the same design.
- Of the facts `remote status` renders, **health** is already carried and
  already drawn: `statusFromRemote` maps it onto `Ready`, and `servingText`
  renders `ReadyNo` as `(not ready)`. That mapping arrived with the gateway's
  wake-a-deployed-node work, which needs to know whether a woken environment
  is serving before routing to it.
- The **base URL** is carried (decomposed into `Engine.Host/Port/Path`) but not
  drawn; the **retention deadline** is on `metrics.Stats`, not
  `daemon.StatusResponse`, so the status fan-out cannot reach it at all.
- `movedTopLevelCommands` and `groupArgs`/`movedSubcommands`
  (`cmd/spinloop/commands.go`) are the two existing signposts: the first for a
  first word, the second for a group subcommand.

## Goals / Non-Goals

**Goals:**

- One command reads a target's status, and one opens a board on it.
- Nothing an operator could see before is lost — including the two facts only
  the cloud path rendered.
- The verbs are thin: the resolver and the renderers already exist, so the
  commands are flag blocks.

**Non-Goals:**

- No `metrics` or `logs`. They carry cloud-only flags (`--cost`,
  `--source`/`--since`/`--instance`) with no daemon counterpart, and need node
  capabilities to survive the move. Their own change.
- No node capability interfaces here. Nothing in this change needs one — see
  D2 for why health and base URL do not.
- No `remote` → `cloud` rename, no `kind: cloud`, no `-f` reversal. A rename
  across every example and doc, deliberately separate.
- No change to what a status reports, how it degrades, or how the dashboard
  behaves once open.

## Decisions

**D1: The verbs are new files registering at the root; the fleet spellings are
deleted, not aliased.**

`status.go` and `dashboard.go` hold the commands. The fleet-scoped builders go,
and their bodies — `renderFleetStatus`, `dashModelFor`, `runDashProgram` —
stay where they are, since they are rendering rather than commands.

Alternative: keep `fleetStatusCmd` and register the same builder at both
levels (rejected — two entries in the tree for one behaviour is the thing this
removes, and the house pattern for a moved command is a hard break that names
the replacement).

**D2: Health and base URL need no new plumbing — both are already carried.**

`remote status` renders facts a daemon has no answer for, and the tempting move
is a `Healther` capability beside `Keeper`, or a fleet-owned status type
wrapping the daemon's. Neither is needed. Health is already mapped onto `Ready`
and already drawn as `(not ready)`, so it costs nothing. The address and the
retention deadline are not drawn — deliberately: each is reported by a command
that exists to report it (`remote env` for the address, `metrics` and the
dashboard for the deadline), and a table read one row per node should not grow
a column only one node kind ever fills.

This keeps the capability interfaces for what they are for — an operation a
node kind can or cannot *perform* (`Keep`, `StartWithProgress`) — rather than a
field one kind happens to know. A capability here would be a second round trip
for a value the first call already returned.

One inherited consequence is worth naming rather than rediscovering: `running`
in `select.go` skips a node reporting `Ready == ReadyNo`, so an unhealthy cloud
environment is not selected for routing. That follows from the health mapping,
which was settled when it landed; this change neither introduces nor alters it.

**D3: A single-node target renders as the table, whatever its node count.**

`spinloop status --env prod` renders the one-row table, not a key-value block:
a command whose output shape depends on how many things it found is harder to
script against and harder to explain than one whose shape is fixed. It reads
the same for one target and for twelve, and it is what the dashboard's detail
view already echoes.

`remote status`'s key-value block is untouched and still available — see the
proposal for why that command stays.

**D4: Two signposts, using the existing mechanism.**

`fleet status` and `fleet dashboard` are group subcommands, so they go in
`movedSubcommands`, which `groupArgs` already reads. No new mechanism.

**D5: The no-target message is fixed here rather than left to the rename.**

`fleet.Resolve`'s failure still offers only `--fleet` and creating a file,
though `--env` has been a target since it landed. With `status` at the top
level that message is what a new user meets first, and this change is what
makes them meet it. Fixing it here also means the spec's "names every way to
give one" requirement lands with the code that satisfies it.

## Risks / Trade-offs

- [An operator who scripted `remote status`'s key-value output gets a table]
  → the layout change is stated in the proposal and the removed spelling names
  its replacement, so the break is loud rather than silent; anything parsing
  status output should be reading `--format=json` from `metrics`, which this
  change does not touch.
- [Health and base URL become fields on `NodeResult` that only one node kind
  fills] → the same is already true of several `daemon.StatusResponse` fields
  (`Engine.Host` is filled by a cloud node and left empty by a daemon), so the
  shape is established rather than new.
- [Two more top-level commands crowd the root] → they replace three
  subcommands, so the total count falls; and `status` is among the words a
  user tries first, which is the point of the move.

## Migration Plan

`spinloop fleet status` → `spinloop status`. `spinloop fleet dashboard` →
`spinloop dashboard`. `spinloop remote status` → `spinloop status --env <name>`.
Each old spelling fails naming its replacement. Nothing is persisted or
transmitted differently; rollback is a revert.
