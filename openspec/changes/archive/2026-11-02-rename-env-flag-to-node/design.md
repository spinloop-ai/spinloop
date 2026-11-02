# Design

## Context

`--env/-e` is registered per-command across `cmd/spinloop` (apply, unapply, the
launch builder in `code.go`, status, logs, dashboard, the fleet group, and the
cloud group), each with its own `StringVar*` call and its own `compRegister(c,
"env", compEnvs)`. The launch path additionally has a separate `--node` pin
(`routeOptions.node`) routed through `routeThroughFleet`, and an explicit
one-target conflict (`checkOneTarget`) between `--env` and any fleet in force.
The fleet machinery already models a registered cloud environment as a
fleet-of-one node (`fleet.ForEnvironment`), so the rename lands on vocabulary
that the internals already use.

## Goals / Non-Goals

**Goals:**
- One flag spelling — `--node`/`-n` — for naming a deployment target on every
  command, with no `--env` accepted anywhere.
- On the launch path, one flag that resolves as fleet-node-pin with a fleet in
  force and as registered environment without one; no silent inertness — the
  old combination `--env` + a directory `fleet.yaml` failed loudly, and the new
  one must also produce a definite outcome (a pin, a gateway route, or a
  failure) rather than launching unconfigured.
- Messages, help, completion, docs and examples speak `--node` without leaving
  "environment variables" wording entangled with it.

**Non-Goals:**
- No rename of the `cloud` package's internal `Environment` vocabulary, the
  on-disk registry (`clouds/<name>/cloud.json`), `cloud env`, `--print-env`,
  `--token-env`, or the `ENV` instruction — those name environment variables,
  where the word is correct.
- No deprecated alias for `--env` (user decision; precedent
  `remote-env`'s "former `-e`/`--env` spelling is removed").

## Decisions

**One flag, two resolutions, decided by fleet-in-force.** The launch keeps a
single `routeOptions` field (`node`, replacing `envName` and the old `node`).
`routeOptions.fleetFile()` already computes "fleet in force"; the launch
branches in `openLaunchCmd.RunE` become: Spinloop worn → `applyBeforeLaunch`
(unchanged); no Spinloop, `--node` set, no fleet in force →
`applyFromEnvironment`; no Spinloop, and either `--fleet` was flagged or
`--node` is set alongside a discovered fleet → `applyFromGateway`; a launch
with no Spinloop, no `--node` and no `--fleet` still applies nothing, as
today. Alternative considered:
keep two flags and alias `--env`'s behaviour behind `--node` plus a separate
pin — rejected: two flags whose validity depends on each other is the
confusion the issue names.

**Cloud fetch keyed off the choice, not the flag.** `applyRoutedSpinloop`
currently fetches the endpoint env whenever `route.envName != ""` and enforces
one-target with `checkOneTarget`. After the merge the flag can name a fleet
node, so the fetch and the provider-keying in `applySelection` run only when
`--node` is set and routing produced no `*fleet.Choice` — i.e. the name
resolved as environment. `checkOneTarget` is removed entirely rather than
re-worded: `--fleet` names a fleet and `--node` names the node within it,
whether the fleet was flagged or discovered, so the pair is one answer, not
two (the first draft refused the pair on the fleet commands and shipped; review
caught that the launch path already accepted it, making the refusal the odd
half). A name a *named* file lacks fails naming the file and its nodes — the
registry answers only beside a discovered `fleet.yaml`, a file that states no
intent.

**`-n` everywhere.** All renamed flags take `-n`. Verified free: only `alias`
takes `-n` (for `--name`), on a command that never took `--env`. fleet-config's
"no short form on the fleet commands" rule is dropped deliberately — the issue
asks for `-n`, and commands outside that group already had the short.

**Launch-slot completion gains names.** `launchSlot` currently answers `--node`
with nothing. With the merge it offers registered environment names
(`compEnvs`) — the no-fleet meaning — and the fleet file's node names where
`fleet.yaml`/`--fleet` is on the line and parses; a failure to read the fleet
degrades to environments only, honouring completion's never-error rule.
`launchValueFlags` gains `-n`. The fleet commands' `--node` gains the same
feeding (`compNodes`): environments plus the fleet in force, an explicit
`--fleet` value beating the directory's file.

**Rename mechanics.** Pure-string renames in specs use MODIFIED; five
requirements whose *scenario titles* carried `--env` (and one whose scenario
titles change because a conflict becomes a pin) go through ADDED+REMOVED, the
pattern this repo used for the `--print-env` rename — the validator pins
MODIFIED scenario titles to the current spec.

## Risks / Trade-offs

- [A user's muscle-memory `--env` now errors] → pflag reports "unknown flag:
  --env"; the release note and docs say the replacement is `--node`. Same
  sharp edge the repo took with `remote`→`cloud` and `-e`→`--print-env`.
- [Rename sweep misses a string] → tests carry most of the surface; grep
  gates (`rg -- '--env|\-e\b' cmd docs examples openspec/specs`) run before
  each commit; the `fleet-docker` CI example exercises real launches.
- [Bare `--node` with a discovered `fleet.yaml` changes behaviour] (was:
  conflict error or silent inertness depending on Spinloop state) → now
  routes or fails; covered by the fleet-routing delta scenarios.
- [`-n` collision in a future flag] → free today on every affected command;
  completion and launch-slot tests pin the current spelling.

## Migration Plan

Single release, breaking. Ship the rename plus a CHANGELOG line naming
`--env`→`--node/-n` and the conflict-rule change. Rollback is reverting the
commit; nothing on disk changes format.
