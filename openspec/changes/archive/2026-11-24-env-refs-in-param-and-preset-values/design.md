# Design

## Context

See proposal.md - Why. The two choke points every consumer goes through are
already in place: `preset.Parse` (called only by `buildServeArgv` in serve.go
and `deployConfig` in cloud.go — the commands that consume presets, fetched
only at consumption time) and `selectionParams` in serve.go (the one
PARAM→layer conversion, called by those same two derivations). `internal/
spinloop` is a pure grammar leaf and stays that way, so `Parse` must not touch
the environment.

## Goals / Non-Goals

**Goals:**

- One expansion implementation shared by both choke points.
- Raw text preserved everywhere the Spinloop itself is re-rendered (`show`,
  `export`, `Format`).
- Values without references byte-identical to today.

**Non-Goals:**

- Resolving references in `MODEL`, `ALIAS`, `CONTEXT`, `OUTPUT`, `PARALLEL`,
  `BASEURL` or `PRESET` values, or in the `ENV` instruction itself — this
  change is PARAM values and preset entry values only.
- Resolving preset **keys** (flag names) or PARAM keys.
- Making plain foreground `serve` apply the Spinloop's `ENV` (it does not
  today); resolution simply uses whatever environment the command holds.

## Decisions

- **New leaf package `internal/envref`** (stdlib only) exporting
  `Expand(value string) (string, error)`. A package rather than a method on
  `Selection`: the spinloop package must stay I/O-free, and `preset` must not
  depend on `spinloop`. Alternative considered: resolve inside
  `internal/spinloop.Parse` — rejected: breaks the leaf invariant and makes
  `show`/`export` leak resolved secrets.
- **`preset.Parse` expands values as it parses**, wrapping the envref error
  with `line %d:` like its other errors; parse already names lines. Expansion
  after `stripComment`/`splitKeyValue`, on the value only.
- **`selectionParams` expands each PARAM value** and grows an `error` return;
  its two callers (`buildServeArgv`, `deployConfig`) propagate. The error names
  the PARAM key (`PARAM hf: …`) because `Selection` carries no line numbers —
  adding them to the grammar for this was weighed and rejected as churn; the
  key plus value shape localises the fault well enough.
- **Expansion syntax**, matching the shell/Compose convention:
  - `$$` → literal `$` (checked first, so `$${FOO}` is `${FOO}` literal).
  - `$NAME` and `${NAME}` → value; unset or empty → empty string.
  - `${NAME:-default}` → value unless unset or empty, else `default`
    (empty default valid).
  - A `$` not followed by a name start, `{`, or `$` stays literal — keeps
    values that merely contain a `$` (prices, regexes) working untouched.
  - Everything else inside `${…}` that the syntax does not define —
    unterminated, bad/empty name, `${NAME:+x}` — is an error. Failing loudly
    beats passing raw text to an engine that would reject it cryptically;
    a lenient passthrough was rejected as it silently mis-serves.
- **Where resolution happens = where the file is read/consumed**:
  `preset.Parse` runs only inside serve/deploy derivation (remote presets are
  fetched there too, resolved against the local environment), and
  `selectionParams` is called only by the two derivations. So `apply`,
  `show`, `export`, completion and the harness paths never resolve, with no
  new skip logic anywhere.

## Risks / Trade-offs

- [A value that genuinely spells `$NAME`/`${NAME}` (e.g. a JS-template-literal
  chat template) now interpolates, silently empty when unset] → Conventional
  and documented; `$$` escapes. JSON chat templates use `{{ }}`/`"` quoting in
  practice; Jinja/llama.cpp templates do not spell shell references.
- [Secrets interpolated into values reach the printed command] → Already
  true of any PARAM value; the existing "Secrets never reach the server
  command" requirement keeps API keys out of scope, and `show`/`export`
  keeping raw text stops resolution leaking into re-rendered files.
- [`preset.Parse` reading the environment makes it environment-sensitive] →
  Tests use `t.Setenv`; the only callers are the two derivations, where that
  is exactly the desired timing.

## Migration Plan

Additive; no migration. Rollback is reverting the commits. Files without
references behave identically.
