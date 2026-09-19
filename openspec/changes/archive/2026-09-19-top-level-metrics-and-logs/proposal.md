## Why

`status` and `dashboard` are top-level verbs serving every target kind.
`metrics` and `logs` are not, and they are the two the last change deliberately
left behind: each carries facts only a cloud environment has, with no daemon
counterpart, so moving them needs the node contract to say what a node kind
can and cannot answer.

- `remote metrics --cost` prices an instance from its type and region. A
  daemon has neither, and `statsFromRemote` drops the instance type on the way
  into the shared stats, so the figure cannot be computed through a fan-out
  today.
- `remote logs --source engine|boot|all`, `--since` and `--instance` query a
  log store. A daemon's log is a byte offset into one file; none of the three
  means anything to it.

Two more facts go with them, because the same commands need them and no
fan-out produces them: an environment's **version**, which the stats reply
carries and the shared stats drop, and the **Spinloop `ENV`** path, which every
`remote` subcommand applies before resolving so that credentials and URLs can
come from a Spinloop's own instructions.

`remote status` is in this change too. It was held back from the last one for
exactly these reasons — a second call for the version, the `ENV` path, the
address and keep figures — and all three read verbs need the same answers.
Solving it once for the three beats solving it twice.

## What Changes

- `metrics` and `logs` become top-level commands, taking their target the way
  `status` and `dashboard` do: `--env <name>`, `--fleet <path>`, or the
  working directory's `fleet.yaml`.
- The node contract gains optional capabilities, asserted by the caller the
  way `ProgressStarter` and `Keeper` already are. A node kind that does not
  implement one is not an error — the flag that needs it reports that this
  target cannot answer it, naming the kinds that can:
  - **pricing** — what a node has cost so far, and its hourly rate, for
    `metrics --cost`.
  - **log queries** — reading a node's log by source, time window and
    instance, for `logs --source`, `--since` and `--instance`.
  - **version** — the spinloop release a node is running, where the node
    reports one outside its status reply.
- **BREAKING** `fleet metrics`, `fleet logs`, `remote metrics`, `remote logs`
  and `remote status` are removed. Each fails naming the command that replaced
  it, the way a moved command already does.
- A Spinloop given to a read verb is read for its `ENV` instructions and the
  `.env` beside it, never to select a target — the rule the `remote`
  subcommands already follow, now stated for the verbs that replace them.
- An environment's endpoint address is not a column of any of these views. It
  is `spinloop remote env`, which prints it eval-safe, and what routing already
  reads.

After this, `remote` holds only what has no fleet counterpart — `bootstrap`,
`auth`, `bake`, `start`, `pause`, `restart`, `stop`, `deploy`, `seed`, `env`,
`ls`, `keep` — and `fleet` holds `route`, `start`, `stop` and `deploy`. No verb
is spelled twice.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `fleet-client`: fleet metrics and fleet logs become top-level verbs serving
  every target kind; the node contract gains the optional pricing, log-query
  and version capabilities, and a flag whose capability a target lacks says so
  rather than failing or silently omitting.
- `remote-stats`: `remote metrics` is removed; what it reported — including
  the cost estimate and the version — is reported by the top-level `metrics`
  against the same environment.
- `remote-logs`: `remote logs` is removed; its source, since and instance
  narrowing become capabilities of the top-level `logs`.
- `remote-endpoint`: the `remote` group no longer has `status`, `metrics` or
  `logs` subcommands, and a Spinloop given to a read verb is read for its
  `ENV` only, as it is for the `remote` subcommands that remain.
- `remote-version-reporting`: an environment's version is read through the
  top-level verbs rather than `remote status` and `remote metrics`.

## Impact

- `internal/fleet`: the three optional interfaces beside `ProgressStarter` and
  `Keeper`; `remoteNode` implements all three, `daemonNode` the version one
  only where its status already carries it. `statsFromRemote` keeps the
  instance type and version it currently drops.
- `cmd/spinloop`: `metrics.go` and `logs.go` hold the verbs; `fleet.go`,
  `fleet_logs.go`, `remote.go` and `remote_logs.go` lose their command
  wrappers and keep their renderers.
- `cmd/spinloop/commands.go`: the two verbs registered, four fleet/remote
  subcommands unregistered, five moved spellings signposted.
- `docs/commands/`: pages for the two verbs; `fleet.md` and `remote.md` point
  at them; every example and CI script updated.
- No change to the fleet file format, the environments registry, the control
  plane, the daemon API, or the gateway.
