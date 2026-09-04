## Why

`--fleet` is the flag that names the fleet file on eight commands, but it has
no short form, while its siblings already do (`--watch/-w` on `metrics`,
`--dry-run/-n` on `deploy`, `--follow/-f` on `logs`, `--spinloop/-O` and
`--harness/-H` on `harness`). Pointing a command at a non-default fleet file is
the most repeated typing of the group, so it should get the same one-keystroke
treatment: `-f`.

## What Changes

- Add `-f` as the short form of `--fleet` on every command that carries the
  flag, one exception: `fleet status`, `fleet metrics`, `fleet start`,
  `fleet stop`, `fleet deploy`, `fleet route`, `fleet dashboard`, and
  `harness`.
- `fleet logs` keeps `-f` as the short form of `--follow`; its `--fleet` stays
  long-form only. Reassigning `-f` there would break the command's most
  characteristic flag (and existing muscle memory and scripts), while adding
  the short form everywhere else is purely additive.
- Tab completion: the `harness` command's custom value-completion logic learns
  `-f` as a value-taking fleet-file flag; flag-name completion needs no
  change because it is derived from the registered tree.

Not breaking: `--fleet` behaves exactly as before everywhere, and no command
loses or repurposes a flag it already had.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `openspec/specs/fleet-config`: the "Fleet file resolution" requirement gains
  the `-f` short form for the fleet subcommands, with the `fleet logs`
  exception made explicit.
- `openspec/specs/fleet-routing`: the "A fleet-routed launch" requirement
  names `-f` alongside `--fleet` as the way to override the Spinloop's
  `FLEET` instruction.
- `openspec/specs/fleet-client`: the "Fleet logs" requirement states that
  `fleet logs` takes its fleet file only as `--fleet`, because `-f` is that
  command's `--follow` short form.

## Impact

- `cmd/spinloop/fleet.go` — the six fleet subcommands register `--fleet` via
  `StringVar`; they move to `StringVarP` with short form `f`.
- `cmd/spinloop/fleet_dashboard.go` — same one-line change for
  `fleet dashboard`.
- `cmd/spinloop/commands.go` — `harness`'s `--fleet` flag gains `-f`.
- `cmd/spinloop/complete.go` — `harnessValueFlags` and the detached-flag
  value case in `harnessSlot` learn `-f`.
- `cmd/spinloop/fleet_logs.go` — unchanged.
- Docs: the flags tables in `docs/commands/fleet.md` and
  `docs/commands/harness.md` list the new short form.
