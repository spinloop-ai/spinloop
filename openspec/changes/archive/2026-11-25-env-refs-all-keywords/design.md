# Design

## Context

`internal/envref.Expand` is the one substitution grammar. Today it is
called from exactly two places: `selectionParams` (cmd/spinloop/serve.go),
the choke point every PARAM consumer passes through, and `preset.Parse`
(resolving preset entry values as they are read). Everything else carries
raw text from `spinloop.Parse` through to the consumer. The proposal widens
the values that resolve to every instruction's value; the parsing-stays-raw
invariant and the PARAM/preset behaviour stay exactly as they are.

Consuming commands read a Spinloop through `readSpinloop`, and the paths
that layer the file's `.env` and `ENV` instructions first call
`applySpinloopEnv` before deriving anything (deploy, wake, seed, gateway,
supervised serve). Plain `serve` and `apply` do not layer ENV before
building their configuration — a value on those paths resolves against the
process environment alone, exactly as PARAMs do today.

## Goals / Non-Goals

**Goals:**

- Resolve references in every keyword's value at the moment a command
  consumes it, once, against the environment that command holds then.
- Keep one choke point per value family so every consumer inherits the
  behaviour and error naming for free.
- Keep `Parse`/`Format`/`show`/`export` byte-raw and `internal/envref`'s
  grammar untouched.

**Non-Goals:**

- No substitution in CLI flag values (`add -m ...`), fleet YAML, `.env`
  files themselves, or the daemon (which reads no Spinloop).
- No nested/recursive references and no rescanning of resolved text.
- No change to how the harness stores provider keys (catalogue `{env:...}`
  references for secrets are unaffected).

## Decisions

### One scalar choke point: `resolveSelection`

A new `resolveSelection(sel spinloop.Selection) (spinloop.Selection,
error)` in cmd/spinloop returns a copy with the eight scalar instruction
values (`Provider`, `Model`, `Alias`, `Context`, `Output`, `Parallel`,
`BaseURL`, `Preset`) resolved through `envref.Expand`, error-wrapped by
keyword (`MODEL: <err>`, `CONTEXT: <err>`, …). It deliberately does **not**
touch `Params` or `Env`:

- PARAM values keep `selectionParams` as their existing single choke point
  (error already names `PARAM <key>`; direct callers of `deployConfigFor`
  and `buildServeArgv` — including the suite — keep resolving through it).
- ENV values are resolved by `applySpinloopEnv` at the moment they are
  exported (see below), so the selection keeps its raw text and nothing
  ever resolves the same string twice.

Resolved text is never re-scanned: the field families are disjoint, so a
value passes through `envref.Expand` exactly once on any command path.
(Rescanning would corrupt values whose expansion legitimately contains a
`$`.)

Call sites — each right after the read, and after `applySpinloopEnv` where
that path has one (so the file's own ENV feeds the values that reference
it): the `apply` and `unapply` bodies, the single launch path in code.go,
`runServe` (before `engineFor`, which consumes `PROVIDER`),
`deriveDeployTarget` (covering `cloud deploy` and `fleet deploy`), the seed
path, `fleet start`/`fleet route`, the gateway's per-node Spinloop read,
and `spinloop alias` (whose default registration name is `ALIAS`). Read
verbs that read a file only for its environment (`read_spinloop_env.go`,
`resolveCloudConfig`) need no scalar resolution — they consume no scalar
value, and their ENV resolution lives in `applySpinloopEnv`.

### ENV values resolve where they are set

`applySpinloopEnv` gains `envref.Expand` per value before `os.Setenv`,
error-wrapped `ENV <KEY>: <err>`. The expansion sees the `.env` overlay
(it runs after pass 1 of the existing two-pass precedence), so
`ENV SLOT=${SLOT_BASE:-1}` fills from `.env` or the process environment and
exports the resolved text. `ENV` keys are never expanded — they reach
`os.Setenv` exactly as written, mirroring PARAM/preset keys.

### Resolution precedes interpretation

Because `resolveSelection` runs right after the read, the resolved text is
already what every later interpretation sees: `engineFor(sel.Provider)` and
the catalogue lookup, `contextsize.Parse` on CONTEXT/OUTPUT,
`parseParallel`, `resolvePresetPath(sel.Preset, …)`, and the
`org/model:quant` split of a resolved MODEL in `deployConfigFor`. No
consumer changes.

### Alternatives considered

- *Resolve inside `deployConfigFor`/`buildServeArgv`.* Rejected: the apply
  path consumes none of those, `PROVIDER` is consumed before both, and
  entry-plus-internal call sites would resolve twice.
- *Resolve everything (scalars, PARAMs, ENVs) in one function at every
  entry.* Rejected: it splits PARAMs off `selectionParams` — a choke point
  direct callers rely on — and double-resolves ENV values that
  `applySpinloopEnv` has already exported.

## Risks / Trade-offs

- A value that legitimately contains `$` (a password in an ENV value, a
  path with `$`) now needs `$$` escaping — the same rule PARAM values
  already impose, widened to all values. Literal-by-default stray `$`
  limits the blast radius to `$`-before-name sequences.
- `apply`/`unapply` pair only when the environment resolves identically;
  a changed variable between the two leaves unapply naming what it looked
  for. Acceptable and consistent: the resolved model id is what the harness
  config actually holds.
- Resolved secrets (e.g. `BASEURL http://$USER:****@…`) can land in harness
  config or deploy payloads as written — the same exposure a literal value
  has today; the substitution itself writes nothing anywhere new.
