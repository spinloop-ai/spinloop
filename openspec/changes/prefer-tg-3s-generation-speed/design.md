# Design

## Context

`ParseGenerationSpeed` (`internal/metrics/speed.go`) turns one captured engine
line into a speed reading. It is one narrow regex over `tg = <n> t/s`, built to
*exclude* `tg_3s` — `\btg\s*=` cannot match `tg_3s =` because the underscore
breaks the `\s*=` — plus a validity filter (unparsable, `NaN`, `±Inf`, or
`≤ 0` is no speed: llama.cpp prints nonsense before a slot's first token). Its
caller is the daemon's `lineTee` → `engineSpeed` fold; `internal/daemon`, the
serve view, and the fleet surfaces all consume the single `Speed` figure, so
this function is the only place the choice of label is made. See proposal.md
for why the reading should move to `tg_3s`.

## Goals / Non-Goals

**Goals:**

- One parsing entry point with unchanged signature — callers, the fold, and
  every wire format are untouched.
- Older llama.cpp builds (no `tg_3s`) and any other engine printing `tg`
  behave exactly as today.

**Non-Goals:**

- No change to the fold, the lapse window, history, or rendering — a `tg_3s`
  reading and a `tg` reading are the same number in the same slot.
- No per-engine label table or configuration: the line's own labels decide.

## Decisions

**Second regex tried first, not one alternation.** Add
`\btg_3s\s*=\s*([0-9]+(?:\.[0-9]+)?)\s*t/s` and try it before the existing
`tg` pattern, sharing the existing validity filter via a small helper (`firstRate(line, re)`). An alternation like `(?:tg_3s|tg)\s*=` would
make the primary pattern's deliberate narrowness a correctness dependency of
the fallback (`tg` inside `tg_3s` must never match, and the "identifier ending
in tg" test exists precisely because that boundary is load-bearing); two
explicit patterns keep each case separately testable and read truer to the
spec sentence "the three-second rate where usable, else the instantaneous
one". Chosen over scanning for both and comparing positions on the line —
label precedence, not column order, is what the spec requires.

**"Usable" decides the fallback, not mere presence.** A `tg_3s` that is
present but unusable (`0.00`, `-nan`) falls through to `tg` rather than
yielding nothing. The alternative — presence wins, so `tg_3s = -nan` kills the
reading even beside a good `tg` — throws away a usable figure the current code
would have read, regressing older-behaviour cases for no spec gain; the
validity filter already treats those prints as the engine not knowing yet, and
"not knowing" is exactly the case the fallback is for.

**Keep `\btg\s*=` byte-identical.** The fallback is the current pattern
unchanged, so every existing negative test (prompt rates, `pp.*t/s`,
identifiers ending in `tg`, `0 t/s`, `-nan`) still pins the same boundary.
`\b` before `tg_3s` gives the same protection for the new pattern: `_` is a
word character, so `n_tg_3s` cannot match.

## Risks / Trade-offs

- [`tg_3s` is noisier than a task average near window edges — a 3 s figure
  can dip while the next batch warms up] → The surface already shows a live
  sparkline with a lapse window; jitter is the honest picture of "now", and
  the fold reports the latest observation, not a smoothed one, exactly as
  before.
- [A future engine label shaped like `tg_3s` (e.g. `tg_30s`) could be read as
  the three-second rate] → `tg_3s\s*=` cannot match `tg_30s =` (the `0`
  breaks the literal); a scenario in the delta covers only the two real
  labels, and any new label is a deliberate spec change.
