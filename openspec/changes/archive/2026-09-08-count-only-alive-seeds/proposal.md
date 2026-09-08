## Why

The seed control surface judges "in flight" differently from the start's
weights gate. The gate counts only seeds whose compute is pending or
running, and the shared discovery module the two Lambdas now build on
documents exactly that — the seed's joined-state logic and the start's
gate count only pending and running, the same way. The seed Lambda does
not do this: its join and its cap count every instance the discovery
returns, stopped ones included.

The two surfaces therefore disagree about a stopped seed's compute. The
gate re-seeds weights whose only seed instance has stopped, while
`spinloop remote seed start` for the same weights reports "a seed is
already running" and waits on it, and a stopped seed keeps its slot
filled against the in-flight cap. The specs agree with the gate: a seed
whose compute has ceased — failed, stopped or reaped — does not count as
running, and only a seed that is already in flight is joined.

## What Changes

- A stopped seed instance is not in flight, on the seed surface too: the
  seed Lambda's join and cap count only pending and running instances,
  the same instances the start's gate counts. A request for weights whose
  only seed has stopped starts a fresh seed, and a stopped seed holds no
  slot against the cap.
- The "is this seed's compute alive" judgement moves next to the
  discovery it interprets: one `seedAlive` in
  `remote/lambda/shared/seed/discovery.ts`, used by the gate, the seed
  Lambda's join and cap, and the state join that reports a seed's status,
  so the surfaces cannot drift apart again.
- The seed Lambda's handler gains its first tests: start (join, cap,
  re-seed, validation), status, list and stop, through the public
  handler with the AWS calls stubbed.

No new commands, flags or replies: the `seed start`, `seed status` and
`seed stop` interfaces are unchanged, and status and stop still address
stopped instances — only the in-flight judgement changes.

## Capabilities

### Modified Capabilities

- `weight-seeding`: a request for a seed that is not in flight — because
  its only instance is stopped — starts a fresh job rather than report
  the ceased seed as running, and stopped seeds do not count against the
  cap on jobs in flight.
