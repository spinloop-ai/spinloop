## Why

A remote environment's `Logs` call ignored the resume position the fleet's
generic follow contract relies on (`fleet-client`'s "Following SHALL resume
each node from the position that node last returned, so a line already
printed is never printed twice"), always re-fetching and re-returning its
whole tail. Because `fleet dashboard`'s detail view and `fleet logs -f`
accumulate what a node returns rather than replacing it, this meant a remote
node's log pane or output repeated its most recent lines on every poll —
visibly, since a slow-loading engine's only output for minutes was the same
one or two startup lines, shown again every 3 seconds. The `remote-node` spec
names "read for logs" as one of the operations a remote environment answers
like any other node, but never states the resumption guarantee that
operation has to meet, so this gap went unwritten as well as unimplemented.

## What Changes

- A remote node's log read now resumes from where it last left off, sharing
  the exact follow cursor (`internal/remote.FollowCursor`, dedup by event id
  over an overlap window) that `spinloop remote logs -f` already used, so the
  two follows behave identically and cannot drift apart.
- The `remote-node` spec gains an explicit requirement for the log-follow
  guarantee a remote environment must meet, naming the shared cursor and the
  "missing" vs. "quiet" distinction (no log ever vs. nothing new since the
  last poll).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `remote-node`: adds a requirement that reading a remote environment's logs
  resumes from the position last returned — via the same follow cursor
  `spinloop remote logs -f` uses — instead of re-reading its tail on every
  poll, and that a from-the-beginning read finding nothing is reported
  distinctly from a later poll finding nothing new.

## Impact

- `internal/fleet/remote_node.go`: `remoteNode.Logs` now holds and uses a
  `*remote.FollowCursor`.
- `internal/remote/follow.go` (new): `FollowCursor` and `FollowOverlap`,
  extracted from `cmd/spinloop/remote_logs.go`'s follow loop so both call
  sites share one implementation.
- `cmd/spinloop/remote_logs.go`: `followLogsLoop` now uses `remote.FollowCursor`
  instead of its own inline dedup map.
- No API or config surface changes; no breaking changes.
