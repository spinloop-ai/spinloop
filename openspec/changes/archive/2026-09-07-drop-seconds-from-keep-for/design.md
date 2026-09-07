## Context

The keep figure is produced by a single function, `formatKeepDuration`
(`cmd/spinloop/metrics_render.go`), reached only through `keepText`. Every
surface that shows the figure goes through it: the dashboard tile and detail
screen (`renderActiveIndented`), the one-shot `fleet metrics` bar body
(`renderActiveIndented`) and table row (`renderActiveKeyValue`), and the
one-shot `remote metrics`. The `json` formats marshal the shared stats struct
and carry `retainUntil` as its RFC 3339 timestamp — they never render the
figure, so they are unaffected. The elapsed figure beside it (`active  2m 5s
ago`) is a different function, `formatDuration`, and is untouched.

See proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**
- The rendered keep figure carries no seconds: hours and minutes, zero units
  dropped.
- Every surface stays worded identically for the same read, per the existing
  `remote-stats` invariant.
- An active keep is always shown, until the deadline actually passes.

**Non-Goals:**
- Changing how a keep is set (the dashboard prompt still accepts any Go
  duration, `4h`, `45m`, `90s`).
- Changing the `json` output, which carries the absolute timestamp by design.
- Changing the elapsed `… ago` figure, whose seconds are meaningful.

## Decisions

**One shared formatter, not a dashboard-only variant.** The tile's wording is
specified to match the one-shot surfaces (`fleet-client`: "worded the same way
the one-shot surfaces report it"). Giving the tile a seconds-free figure while
the one-shot kept seconds would break that invariant, so the granularity rule
lives in the one function all surfaces share. The user pointed at the tile
because that is where the churn is visible; the fix belongs upstream of it.

**Sub-minute case: floor to the minute, minimum `1m`.** When less than a
minute remains there are three defensible readings — round to the nearest
minute (which yields `keep for 0m` at the tail and needs a new omission rule),
ceil to the next minute, or floor with a `1m` floor. Floor-with-`1m`-floor is
chosen because it never overstates the time left and it keeps the figure
present for any future deadline, so the existing omission rule ("absent only
once the deadline has passed") needs no new condition. A keep of `45s` renders
`keep for 1m`; `2h 0m 30s` renders `keep for 2h`.

**Keep the existing `Round(time.Second)` entry step.** The function already
rounds to the second before decomposing; keeping it means the minute value is
a clean floor of the true remainder rather than an artefact of float duration
math.

## Risks / Trade-offs

- [A sub-minute keep shows `keep for 1m` for up to a minute, which slightly
  overstates the time left at the very tail] → Accepted: the alternative
  (the figure vanishing mid-keep, or reading `0m`) is more confusing, and the
  node is about to be swept anyway.
- [Any code or test that pinned a seconds-carrying keep figure breaks] → None
  exists: the rendered figures asserted in `retain_render_test.go` and
  `dashboard_keep_test.go` already carry no seconds, so they stand; new
  coverage pins the sub-minute and whole-unit cases.

## Migration Plan

No data, API, or control-plane change; a single commit to the renderer and its
tests. Rollback is reverting the commit.
