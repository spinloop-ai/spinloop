## Why

Reading an engine's state is one operation, and the CLI spells it twice.
`spinloop fleet status` and `spinloop remote status` answer the same question
about the same kind of thing, and since `--env` landed both work on the same
environment and print differently. The renderers converged long ago —
`status_render.go` exists because someone noticed — but the commands above
them did not.

The split makes the operator pick a command by how a target happens to be
configured rather than by what they want to know. A cloud environment is
`remote status`; the same environment named in a fleet file is `fleet status`;
the dashboard already draws both kinds side by side and does not care.

Everything needed to collapse them is in place: a cloud environment is reached
through the same `fleet.Node` contract a daemon is, one resolver already turns
`--env`/`--fleet`/the working directory into the fleet to act on, and the
fan-out renders a mixed fleet without knowing which kind each node is. What is
left is to put the verb at the top level and delete the duplicate.

This change moves the two read verbs that need nothing new to do it. `metrics`
and `logs` follow in their own change, because they carry cloud-only facts
(`--cost`, `--source`/`--since`/`--instance`) that have no daemon counterpart
and need node capabilities to survive the move. Splitting on that line keeps
this change mechanical and gives that design its own review.

## What Changes

- `status` and `dashboard` become top-level commands. Each takes its target
  the way the fleet commands do — `--env <name>`, `--fleet <path>`, or the
  working directory's `fleet.yaml` — and renders exactly as its fleet-scoped
  spelling does today.
- **BREAKING** `spinloop fleet status`, `spinloop fleet dashboard` and
  `spinloop remote status` are removed. Each fails naming the command that
  replaced it, the way a moved command already does.
- **BREAKING** With no target resolvable — no `--env`, no `--fleet`, no
  `./fleet.yaml` — the verbs fail rather than looking for an engine on this
  machine. `spinloop serve` already shows the engine it runs, and a machine
  that wants the full set locally runs `spinloop daemon` and names it in a
  fleet file. The failure names all three ways to give a target, which today's
  message does not: it offers only `--fleet` and creating a file, though
  `--env` has been a target since it landed.
- What `remote status` reported that a node's status does not carry — the
  endpoint's health and its address — is carried into the top-level verb, so
  nothing an operator could see is lost with the command.
- `fleet` keeps `route`, `start`, `stop`, `deploy`, `metrics` and `logs`;
  `remote` keeps everything else it has. Only `status` and `dashboard` move
  here.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `fleet-client`: fleet status and the fleet dashboard become top-level verbs
  serving every target kind, keeping their rendering, their degradation
  behaviour and the dashboard's keys unchanged.
- `fleet-config`: target resolution is stated for the top-level verbs as well
  as the fleet group, and the failure with no target names all three ways to
  give one.
- `remote-endpoint`: the `remote` group no longer has a `status` subcommand,
  and what that subcommand reported — state, health, last-active — is reported
  by the top-level `status` against the same environment.
- `remote-version-reporting`: the version an environment reports is read
  through the top-level `status` rather than `remote status`.

## Impact

- `cmd/spinloop/status.go` and `dashboard.go`: the two verbs, registered at the
  root, resolving through the existing `resolveFleetTarget`.
- `cmd/spinloop/fleet.go`, `fleet_dashboard.go`: lose their command wrappers,
  keep their renderers and the dashboard model.
- `cmd/spinloop/remote.go`: loses `remoteStatusCmd` and `runRemoteStatus`; the
  health and address facts it rendered move into the shared status view.
- `cmd/spinloop/commands.go`: the two verbs registered, the two fleet
  subcommands unregistered, and the moved spellings signposted — `fleet
  status`, `fleet dashboard` and `remote status` each naming their
  replacement.
- `internal/fleet/config.go`: the no-target failure names `--env` too.
- `docs/commands/`: pages for the two verbs; `fleet.md` and `remote.md` point
  at them.
- No change to the fleet file format, the environments registry, the control
  plane, the daemon API, or the gateway.
