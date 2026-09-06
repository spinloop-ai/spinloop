# State-aware dashboard key hints

## Why

The dashboard's key help line names `s start` and `x stop` for the node under
the cursor whatever that node's state is, so it invites the operator to press
a key that would do nothing there: starting a node that is already running,
stopping one that is not. The abort and keep hints already follow the rule the
spec sets for them — never name a key that would do nothing for the node the
hint describes — and start and stop do not. (issue #173)

## What Changes

- The dashboard's key help line — the grid's footer for the node under the
  cursor, and the detail view's footer for the node in view — names `s start`
  only when that node is not running on its current board read, and names
  `x stop` only when it is. A node whose state is unknown — no answered read
  yet, or its newest read failed — still shows start, and not stop: there is
  no reason to withhold the one key that might do something.
- Neither key is named while that node has an action in flight (a start, a
  stop, or a keep), the same gate the keep hint already carries and the same
  guard the keys themselves answer to, since an in-flight action leaves both
  driving nothing.
- A node that never became a node — a token reference that resolves to
  nothing — names neither key, since neither would drive anything on it.
- The keys themselves are unchanged: `s` on a running node and `x` on a
  stopped one still drive nothing and say so. Only what the hint line
  advertises changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `fleet-client`: the requirement "The dashboard drives the selected node"
  gains the rule for when the key help names the start and stop keys,
  parallel to its existing rule for the abort key, with scenarios.

## Impact

- `cmd/spinloop/dashboard_model.go`: the grid and detail key lines are built
  from per-key offers for the node the line describes, in the style of the
  existing `keepOffered` and `canAbort`.
- `cmd/spinloop/dashboard_render.go`: the abort-only hint filter is folded
  into the key line's construction.
- `cmd/spinloop` tests: the key help tests in `fleet_dashboard_test.go` and
  `dashboard_keep_test.go` are extended to the state-by-action matrix.
- `docs/commands/fleet.md`: the dashboard's key table gives `s` and `x` the
  same shown-only-where-they-drive-something wording `k` already carries.
- No API, file format, or dependency changes; no node operation changes.
