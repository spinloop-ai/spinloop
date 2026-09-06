# Design: the `up` command

## Context

`up` starts the engine for what is in the current directory. Two things define
the directory:

- A `fleet.yaml` in it — a fleet of machines. `spinloop fleet start` drives
  those nodes; it resolves its fleet file with `fleet.Resolve` (default
  `./fleet.yaml`), and `runFleetDrive` refuses to run bare — it needs a node
  name or `--all` (`cmd/spinloop/fleet.go:366`).
- A resolvable Spinloop — a local engine. `spinloop serve` builds and runs it;
  it resolves its source with `readSpinloop` (`cmd/spinloop/main.go:311`),
  which tries, in order: an explicit path argument, a registered alias
  (`resolveAlias`), the `SPINLOOP_ALIAS` environment variable
  (`spinloopFromEnv`), then `./Spinloop`.

`up` is a thin dispatcher over those two existing commands. It owns no engine
logic of its own: whatever it runs, it runs through the command it delegates
to, so the two cannot drift apart.

## Goals / Non-Goals

**Goals:**

- `up` starts the right thing from the directory: the fleet when a
  `fleet.yaml` is present, the local engine otherwise.
- `up`'s serve path is `serve` itself — same resolution, same flags' meaning,
  same errors — so a directory where `serve` works is a directory where `up`
  works.
- The quick path stays quick: one word, no flags.

**Non-Goals:**

- No `down` or other counterpart: each gets its own change.
- No flags on `up`, no change to `serve` or `fleet start`, and nothing for the
  alias registry.

## Decisions

**1. A command of its own, not a Cobra alias.**

A Cobra `Aliases` entry dispatches to one fixed target; `up`'s target depends
on the working directory, so it needs its own `RunE` to choose. `upCmd()` goes
in a new `cmd/spinloop/up.go`, registered beside `serveCmd()` in
`cmd/spinloop/commands.go`. It is `cobra.NoArgs`-free — it takes the positionals
it forwards and nothing else.

**2. The dispatch is two checks, fleet first.**

1. A `fleet.yaml` exists in the current directory → the fleet branch. A
   Spinloop is ignored entirely when a fleet file is there: a fleet directory
   is a fleet, and the rule is one to state in a sentence.
2. Otherwise → the serve branch.

The fleet check is the presence of `./fleet.yaml` — the same file
`fleet.Resolve("")` would read — so `up` and `fleet start` agree on what a
fleet directory is without `up` re-parsing the file to decide.

**3. The serve branch is serve.**

`up [path]` calls `serve`'s own body with the positional it was given (nothing
when none). There is deliberately no separate "is a Spinloop available?"
pre-check: the availability decision and the run are the same resolution, so
`up` and `serve` cannot diverge. A directory where `serve` works — say via
`SPINLOOP_ALIAS` or a registered alias — is a directory where `up` works, and
a directory where it does not fails with `serve`'s own "no Spinloop found"
error, which already names the repairs.

**4. The fleet branch is fleet start, with bare meaning all.**

`up [node…]` runs the fleet start path over the named nodes; with no
positionals it starts every node — the `--all` target — because a bare
`fleet start` refuses to guess. Unknown node names, an unparseable or
node-less `fleet.yaml`, and the per-node result lines are `fleet start`'s own.

**5. Positionals only, no flags.**

`up` forwards positionals and nothing else. A flag set shared by both branches
would make one flag mean different things per directory — `--all` to the
fleet, nothing to serve; `-n` to serve, nothing to the fleet. The full options
stay one verb away: `fleet start` and `serve` directly.

**6. Completion offers what the branch accepts.**

`up`'s completion slot offers the fleet's node names when `./fleet.yaml` is
present — read from the file, silently when it cannot be, as the completion
protocol requires — and otherwise the usual Spinloop slot, registered alias
names plus paths. `fleet start` itself offers no positional candidates today;
`up` does, because its whole job is to remove a step. The `shell-completion`
spec needs no delta: its coverage requirement is derived from the command
tree, which picks `up` up automatically.

## Risks / Trade-offs

- [A broken `fleet.yaml` shadows a valid Spinloop in the same directory] →
  by design (fleet wins); the failure is the fleet file's own parse or
  validation error, which names the file, so the shadowing is visible.
- [A user in a fleet directory expecting `serve`] → `up`'s help states the
  rule in a sentence, and `serve` still does exactly what it always did.
- [A bare `up` in a fleet directory starts every node, which can be expensive]
  → the agreed "bring it up" default; starting a subset is `up <node>`, and
  the per-node result lines show exactly what started.

## Migration Plan

Additive; nothing to migrate. Rollback is removing the command and its
registration.
