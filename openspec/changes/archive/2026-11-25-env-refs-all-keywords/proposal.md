# Proposal: Environment references in every keyword's value

## Why

Environment references (`$FOO`, `${FOO}`, `${FOO:-bar}`, `$$`) are currently
resolved only in `PARAM` values and preset entry values. A Spinloop that
parameterises itself per machine — a model tag, a base URL host, a context
size — can carry those only in engine params, so every other instruction
(`MODEL`, `BASEURL`, `CONTEXT`, the `PRESET` path, even `ENV` values) still
has to be duplicated per environment by hand. Users expect one substitution
language across the file, the way Docker Compose and shell conventions work.

## What Changes

- Environment references resolve in the value of every Spinloop instruction,
  not just `PARAM` and preset entries: `PROVIDER`, `MODEL`, `ALIAS`,
  `CONTEXT`, `OUTPUT`, `PARALLEL`, `BASEURL`, the `PRESET` path, the `VALUE`
  half of an `ENV KEY=VALUE`, and (as today) `PARAM` values and preset entry
  values.
- Names and keys stay literal: instruction keywords, `PARAM` keys, preset
  entry keys and `ENV` keys are never resolved (extending the existing rule
  to `ENV` keys explicitly).
- Resolution still belongs to the consuming command — `Parse`, `show` and
  `export` keep carrying and rendering the raw text unchanged. Commands that
  act on the file (`serve`, `harness apply`/`unapply` and the launch path,
  `cloud deploy`/seed and a node wake, fleet start/route, gateway node
  reads, `alias` registration) resolve against the environment they hold at
  that moment.
- Where a value feeds another grammar, the resolved text is what gets
  interpreted: `CONTEXT`/`OUTPUT`/`PARALLEL` numbers are parsed after
  resolution, `PROVIDER` after resolution resolves the catalogue lookup, the
  `PRESET` path after resolution resolves against the Spinloop's directory,
  and an `ENV` value after resolution is what gets exported, so the file's
  own `ENV` keeps feeding other values exactly as it feeds `PARAM`s today.
- A malformed reference in any value fails the consuming command naming the
  offending keyword (its `PARAM` key or preset line, as today), rather than
  passing raw text on.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `spinloop-files` — the "Environment references in values" requirement
  widens from PARAM/preset values to every instruction's value, states that
  keys and keywords are never resolved (now including `ENV` keys), and that
  resolution precedes any interpretation of the resolved text.

## Impact

- `internal/spinloop` — parsing stays raw; doc comments on `Selection`/
  `Param` update to the widened rule.
- `cmd/spinloop` — a resolution choke point (alongside `selectionParams`)
  applied by every command that consumes a Spinloop for an action
  (`serve.go`, apply/unapply/launch in `main.go`, `cloud.go`, `cloud_seed.go`,
  `fleet.go`, `gateway.go`, `alias`/`read_spinloop_env.go` paths);
  `applySpinloopEnv` resolves `ENV` values before exporting them.
- No change to `Parse`/`Format` behaviour (raw in, raw out), to preset
  parsing, or to `internal/envref`'s grammar.
- Docs: the Spinloop file-format pages under `docs/` describe PARAM-only
  substitution and must be updated to the whole-file rule.
