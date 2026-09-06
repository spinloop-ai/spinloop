# Dashboard keep

## Why

The dashboard can start and stop a fleet's nodes, but the one fact an operator
needs while watching a remote environment — until when the cloud's idle sweep
will leave it alone — is neither settable nor visible there. Keeping a node
alive (overnight debugging, a long-running task) means leaving the dashboard
and running `spinloop remote keep` one-shot, and the deadline it sets then
appears nowhere in the view the operator is actually watching. Two open issues
ask for the two halves: #158, set a user-specified keep period from the
dashboard, and #157, show the keep-until time on the node's detail screen.

## What Changes

- The dashboard gains a keep action on the selected node: a key opens a
  duration prompt (a hand-rolled single-line field in the existing model — no
  new dependencies), and confirming sends the node's retention deadline to now
  plus the entered duration, through the same control-plane path
  `spinloop remote keep` already uses.
- Keep is offered only where it can work: `kind: remote` environments, through
  a node capability in the style of the existing progress-reporting start. The
  key help hides the key for local daemon nodes, exactly as it already hides
  the abort key where it would do nothing.
- The fleet's read of a remote environment carries the retention deadline: the
  stats Lambda already parses the instance's Retain-Until tag and now includes
  it in its reply, and the client maps it onto the shared stats shape. Older
  control planes omit the field, and the deadline line simply does not appear
  — the same graceful degradation the stats reply's other relayed facts use.
- The dashboard's node panel shows the deadline — the tile and the full-screen
  detail screen draw the same lines, so both show it — whenever a read carries
  one, and omits it when a node has no active retention. The line is the
  shared one the bar format prints, so the one-shot `fleet metrics` and
  `remote metrics` render it the same way.
- Keep proceeds without confirmation — it overwrites the deadline and ends
  nothing — and is not abortable: it is a single fast call, like a stop.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `fleet-client`: the dashboard gains a keep action (the duration prompt, its
  validation, its in-flight and outcome handling, and the key help), and the
  node panel — tile and detail screen — shows the retention deadline a read
  carries.
- `remote-stats`: the stats Lambda's reply carries the instance's retention
  deadline when the Retain-Until tag is present, so every surface that reads
  the environment's stats can show it.

## Impact

- `internal/fleet`: a new capability interface, implemented by the remote node
  over the existing `remote.Keep` call; local daemon nodes do not implement it.
- `internal/remote`: the stats reply type gains the deadline field; the node's
  stats mapping carries it across.
- `internal/metrics`: the shared stats shape gains the deadline field.
- `remote/lambda/stats`: the reply includes the tag the Lambda already parses.
- `cmd/spinloop`: the dashboard model (prompt state and keys, the action), the
  shared line renderer, and the command's help text.
- `docs/commands/fleet.md`: the dashboard's keys.
- No new dependencies. The control-plane change is additive: an environment on
  an older deployment reads back without the field and shows no line.
