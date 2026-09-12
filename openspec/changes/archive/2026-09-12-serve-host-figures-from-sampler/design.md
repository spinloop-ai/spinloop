## Context

The daemon's sampler already takes one system reading per tick for the retained
history, and the metrics handler collected the same host figures again on every
request. See proposal.md for why the second collection had to go.

## Goals / Non-Goals

**Goals:**

- One collection per tick serves both the retained history and the reported
  reading.
- A broken host source stays visible: its failure is reported somewhere a
  caller sees.

**Non-Goals:**

- The engine's token counters: they already come from the sampler's reading,
  and this change does not touch how they are reported.
- The sampler's interval: it stays at its own interval; only what a reading
  serves changes.

## Decisions

### The handler copies, never collects

The daemon keeps the most recent system reading: the figures, the collection's
failures, and whether a reading has landed. The sampler records each reading
there; the metrics handler copies it onto the response. Before the first
reading the handler copies nothing, so an unsampled figure stays absent rather
than reading as a host with no CPU. A start after a stop drops the reading, so
the new engine does not report the last engine's host.

The cost is staleness bounded by the sampling interval. For utilisation figures
in a refreshing view, a reading at most one tick old is not a cost: the view is
already a sample of a moving host, and the alternative — collecting inline —
made the endpoint slow in proportion to how busy the host was, which is
backwards for a watched machine.

### The reading carries the collection's failures

With no collection taken on the request, the handler has no errors of its own.
A failed reading is recorded as the current reading with its failures, and the
handler reports them among the response's errors. A failed reading still
contributes no figures to the retained history: there, a failure is a
non-observation, as in the activity record.

### The catch-up interval bounds itself with a scrape target

The short interval that runs until the first engine sample exists applied
whenever no counters had come back. For an engine whose runner exposes no
metrics endpoint, counters never come back, so the sampler would have run the
host commands every second for the engine's whole life. The interval now
applies only while a scrape target is known: no target, the sampler settles at
the tick.

### The dashboard's cadence matches the data's

The board re-reads the local daemons every 5 seconds rather than 2: it is one
call per machine per tick, and the figures it draws are sampled every 15
seconds, so polling faster re-fetches readings that have not changed.

## Risks / Trade-offs

- A caller who polls metrics faster than the tick sees figures that do not move
  between ticks. That is the sampler's contract already, and the dashboard is
  the only in-repo caller that relies on these figures refreshing.
- A host source that breaks between ticks is visible once a tick's reading
  fails — at most one tick later than an inline collection would have shown
  it.
