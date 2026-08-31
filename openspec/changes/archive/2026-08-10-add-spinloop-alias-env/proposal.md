## Why

An alias saves typing a path, but only if you type the alias — every command
still needs `qwen3.6-27b` spelled out, or a `cd` into the directory that holds
the `Spinloop`. There is no way to say "this shell is wearing qwen" once and have
`apply`, `serve`, `daemon` and `remote` all agree. Every other machine-local
runtime choice already has an environment variable (`SPINLOOP_HARNESS`,
`SPINLOOP_BASE_URL`, `SPINLOOP_PROVIDERS`, `SPINLOOP_CONFIG_DIR`); the Spinloop itself
is the one missing.

## What Changes

- Add `SPINLOOP_ALIAS`: a registered alias name that stands in for the `[path]`
  argument when a command is given none. `export SPINLOOP_ALIAS=qwen3.6-27b` then
  makes `spinloop apply`, `spinloop serve`, `spinloop daemon` and the `remote`
  subcommands act on that Spinloop from any directory.
- Precedence for the Spinloop a command acts on: explicit path or alias argument,
  then `SPINLOOP_ALIAS`, then `./Spinloop`. The variable changes **which** Spinloop is
  the default — it never causes a command to act on a Spinloop it would
  otherwise have left alone.
- `SPINLOOP_ALIAS` holds a registry name only, never a path, and is looked up
  directly: unlike an argument of the same spelling, a same-named file in the
  working directory does not shadow it. A value that is not registered, or that
  points at a file that has gone, fails naming the variable instead of dropping
  through to a confusing "no such file".
- When `SPINLOOP_ALIAS` decides the Spinloop, the command says so on stderr, the
  same way an alias argument already does.
- `spinloop alias [path]` is excluded: its bare form means "register the Spinloop
  here", so honouring the variable there would only ever re-register what is
  already registered.
- Documentation: `README.md`, `docs/commands/alias.md`, the command pages that
  take a path, and `AGENTS.md`.

## Capabilities

### New Capabilities

None — this extends the existing alias registry rather than introducing a
capability of its own.

### Modified Capabilities

- `alias-registry`: alias resolution gains an environment source — a new
  requirement covering `SPINLOOP_ALIAS`, its direct (unshadowed) lookup, its
  error cases and its stderr report.
- `spinloop-files`: the "Spinloop path resolution" requirement gains the
  `SPINLOOP_ALIAS` step between an explicit argument and the `./Spinloop` default,
  and records that `spinloop alias` and a bare `spinloop harness` are unaffected.

## Impact

- `cmd/spinloop/main.go` — `readSpinloop`, the single choke point every Spinloop
  command shares, gains the environment fallback; `cmdAlias` opts out of it.
  The not-found hint for a missing `./Spinloop` mentions the variable.
- Behaviour of `spinloop apply`/`unapply`/`serve`/`daemon`/`harness --spinloop`/
  `remote *` when no path is given — only in a shell that sets the variable.
- Tests: `cmd/spinloop/alias_test.go` (resolution, precedence, errors),
  plus coverage that a bare `spinloop harness` and `spinloop alias` stay unchanged.
- No new dependencies; `internal/config` is unchanged.
