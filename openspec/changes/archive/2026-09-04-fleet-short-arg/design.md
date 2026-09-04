## Context

See proposal.md for motivation. The relevant current state:

- Eight commands register a value-taking `--fleet` flag: the six `fleet`
  subcommands `status`, `metrics`, `start`, `stop`, `deploy`, and `route`
  (all in `cmd/spinloop/fleet.go`), `fleet dashboard`
  (`cmd/spinloop/fleet_dashboard.go`), and `harness`
  (`cmd/spinloop/commands.go`).
- `fleet logs` is the ninth `--fleet`-bearing command, but its `-f` is
  already the short form of `--follow` (`cmd/spinloop/fleet_logs.go`). `remote
  logs` uses the same `-f`/`--follow` pairing but has no `--fleet` flag, so
  it is unaffected.
- The flag framework (pflag, via Cobra) forbids two flags on one command
  sharing a short name; a second registration of `-f` on `fleet logs` panics
  at command-tree construction.
- Completion is derived from the registered tree, so flag *names* in both
  forms complete without any table. The one custom table is
  `harnessValueFlags` in `cmd/spinloop/complete.go`, which the `harness`
  command's hand-rolled slot logic uses to know which flags consume a
  following word, plus the detached-flag value case in `harnessSlot`.

## Goals / Non-Goals

**Goals:**

- `-f` means the fleet file on every `--fleet`-bearing command except
  `fleet logs`, where it keeps meaning `--follow`.
- `--fleet` keeps working everywhere, unchanged, so no script or muscle
  memory breaks.
- Completion keeps working: `-f` completes as a flag name (automatic) and
  completes its value as a file path (the one table that needs to learn it).

**Non-Goals:**

- No reassignment of `--follow` on `fleet logs`; the exception is the
  design, not a gap to close.
- No other flag renames or new short forms.
- No change to how the fleet file is resolved once the flag is read;
  `fleet.Resolve` and its callers are untouched.

## Decisions

**1. The exception list is exactly `fleet logs`, and the short form is not
added there.**

Alternative considered: give `--fleet` the `-f` short form on `fleet logs`
too, by moving `--follow` to another short letter. Rejected: following is
the defining verb of `logs`, `-f`/`--follow` is already written into specs,
docs, and examples, and changing it would be a breaking change to buy
nothing — the other eight commands cover the typing this issue is about.
Leaving `logs`'s `--fleet` long-only is additive-only on every command.

**2. Register with `StringVarP(..., "f", ...)` at each existing call site,
not a shared helper.**

Each command already registers its own `--fleet` with its own help string
(`fleetFileUsage` shared by the fleet subcommands; a fleet-routing-specific
string on `harness`). A helper would save nothing — the short name is a
constant argument on an existing one-line call — and it would centralise the
one place a future flag collision could hide. The six fleet subcommands,
`dashboard`, and `harness` each change one line from `StringVar` to
`StringVarP` with `"f"`; `fleet logs` keeps `StringVar`.

**3. Completion: teach only `harnessSlot` about `-f`.**

For the fleet subcommands, nothing custom sees the flag: the value
completion is `compFiles` registered per command (file paths, no static
candidates), and flag names come from the tree. `harness` is different —
Cobra flag parsing is off for it (`SetInterspersed(false)` plus manual
positional handling), so `harnessSlot` counts words itself and needs `-f` in
`harnessValueFlags` to know `-f <word>` consumes that word; its
detached-flag case must offer the file-path directive for `-f` just as it
does for `--fleet`.

**4. Tests mirror the spec's scenarios.**

- A fleet subcommand with `-f <path>` uses that file (one representative
  command through the existing stub-node harness, since all eight share
  `fleet.Resolve`; the flag wiring itself is checked for every command by
  the flag-surface walk the suite already does, and by a direct
  "does this command accept `-f`" check on each of the eight).
- `fleet logs -f` is follow, not a fleet file: `logs -f --fleet <path>` runs
  against that file in follow mode, and a bare `-f <something>` treats
  `<something>` as a node name (unknown-node error), which is the
  specification's observable consequence.
- `harness -f <path>` routes through that fleet file — the existing
  route-test harness already drives `harness` with `--fleet`; the `-f`
  variant is the same test with the short flag.
- Completion: `harness -f <TAB>` offers file paths (directive default);
  `-f` appears in each command's offered flag names via the existing
  tree-walk guard.

## Risks / Trade-offs

- [`logs` is the odd one out] → An operator who types `fleet logs -f
  ./other.yaml` gets an unknown-node error naming `other.yaml`, which is the
  correct interpretation of what they typed; the docs' flags table and the
  `fleet logs` help state the exception, and the specs pin it with a
  scenario.
- [A future command could silently re-collide on `-f`] → pflag panics at
  tree construction, so the failure is loud and at startup, never at the
  user's keypress; the build and the existing command-construction tests
  catch it.
- [Help text drift: `harness --help` and the fleet subcommands' help now
  show `-f, --fleet`] → The help strings are the existing ones; pflag renders
  the short form automatically. No hand-written help text names the flag,
  so there is nothing to keep in sync in code; the docs tables are updated
  in the same change.

## Migration Plan

Nothing to migrate: the change is purely additive on seven commands plus one
(`harness`) and leaves `fleet logs` byte-identical. Rollback is reverting
the change; no state, config file, or on-disk format is touched.
