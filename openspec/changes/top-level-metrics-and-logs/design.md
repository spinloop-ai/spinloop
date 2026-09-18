## Context

See proposal.md — Why. The implementation-relevant current state:

- `resolveFleetTarget` (`cmd/spinloop/target.go`) already turns `--env` /
  `--fleet` / the working directory into the `*fleet.Config` to act on, and
  `status` and `dashboard` already use it. These verbs are two more callers.
- `fleet.Node` already carries two optional capabilities — `ProgressStarter`
  and `Keeper` — each asserted by its caller with a fallback. The pattern, the
  doc-comment shape and the "a caller that can offer it asserts for it" rule
  are established.
- `statsFromRemote` (`internal/fleet/remote_node.go`) maps the cloud stats
  reply onto `metrics.Stats` and **drops four fields it carries**:
  `Version`, `InstanceType`, `InstanceID` and `Environment`. The first two are
  exactly what the cost and version want.
- `remote metrics --cost` computes from `resp.InstanceType`, `resp.UptimeSeconds`
  and `cfg.Region`, looking the price up through the AWS Price List API and
  silently omitting the line when the lookup fails.
- `remote logs` queries a log store: `--source engine|boot|all`, `--since`,
  `--instance`, over `remote.LogQuery`. `fleet logs` reads a byte offset into
  one file. The two share `--follow`, `--limit` and `--format`.
- Every `remote` subcommand applies a Spinloop's `ENV` instructions and the
  `.env` beside it before any control work (`resolveRemoteConfig`), so AWS
  credentials and `SPINLOOP_REMOTE_*` values can come from a Spinloop. The
  top-level verbs have no such path today.

## Goals / Non-Goals

**Goals:**

- One command reads an engine's metrics, and one reads its logs, whatever kind
  of thing is running it.
- A fact only one node kind has stays available, without a caller branching on
  kind.
- Nothing an operator could get from the removed commands is unreachable.

**Non-Goals:**

- No `remote` → `cloud` rename, no `kind: cloud`, no `-f` reversal. A rename
  across every example and doc; its own change.
- No change to what the stats reply carries, to the log store query, or to the
  daemon's log endpoint. The capabilities expose what already exists.
- No new output format, and no change to the existing four.

## Decisions

**D1: Three capabilities, each a node-kind ability rather than a reply field.**

`Coster`, `SourceLogger` and `Versioner` join `ProgressStarter` and `Keeper` on
`fleet.Node`. A kind that cannot answer does not implement one; a caller that
offers the flag asserts for it and leaves the node as it reads without it.

The alternative — widening `metrics.Stats` with an `InstanceType` and letting
the renderer price it — was rejected twice over. It puts a cloud-only field on
the shared reply that every daemon leaves empty, and it puts the price lookup
(a network call to the AWS Price List API) in a renderer. `Coster` keeps both
where they belong: the node knows its type and region, so it answers *what did
this cost*, not *what type are you*.

**D2: A flag whose target cannot answer it renders blank, and does not fail.**

`metrics --cost` over a fleet of daemons shows no cost and succeeds. This is
what these views already do for every fact only one kind reports — the
readiness mark, the version — and a flag does not make it a different case. A
fleet with no priceable node is a fleet with no cost to show, not a malformed
command line.

Alternatives: refuse up front when no node can answer (rejected — it makes a
flag's validity depend on the target's contents, and a mixed fleet still needs
this rule anyway); warn on stderr (rejected as ceremony for a column that is
blank for a documented reason).

The cost of D2 is that a blank column does not explain itself, so the
requirement obliges each verb's page to say which flags apply to which kinds.

**D3: `statsFromRemote` stops dropping the version and instance type.**

Both are already in the reply the cloud node receives. `Versioner` reads the
version from the node's retained reply rather than making a call, so the
version costs nothing on the metrics path — unlike on `status`, where the
version lives in a different endpoint and was what made `remote status` hard to
move in the first place.

`metrics.Stats` gains no field: the node keeps what it needs to answer its own
capabilities, which is D1 applied consistently.

**D4: The Spinloop `ENV` path moves to the verbs, as an argument that selects
nothing.**

A Spinloop given to a read verb is read for its `ENV` instructions and adjacent
`.env`, never to select a target — the rule the `remote` subcommands already
follow. Without it, an operator whose AWS credentials or `SPINLOOP_REMOTE_*`
values come from a Spinloop loses them when the command they used is removed.

This is the piece that made the last change stop short of `remote status`.
Implementing it once here serves all three removed reads.

**D5: Five signposts, through the existing mechanism.**

`fleet metrics`, `fleet logs`, `remote status`, `remote metrics` and
`remote logs` go in `movedSubcommands`, which `groupArgs` already reads. The
`remote` group gains the same `Args: groupArgs` the fleet group has.

## Risks / Trade-offs

- [A blank column reads as "zero" rather than "not applicable"] → D2's
  accepted cost, mitigated by the documentation the requirement obliges. The
  alternative rules were worse: one makes a flag's validity depend on what the
  fleet happens to contain, the other prints ceremony on every run.
- [Three capabilities at once is a lot of new interface] → they are three
  instances of a pattern already in the tree twice, each with exactly one
  implementor and one caller. The shape is not new; the count is.
- [The price lookup is a network call inside a fan-out] → it already is one,
  on the `remote metrics` path; `Coster` moves it behind the node rather than
  adding it. It stays behind `--cost`, so a run that does not ask pays nothing.
- [Removing five commands at once is a wide break] → each names its
  replacement, and they are the last five duplicate spellings; leaving any
  behind would mean a second change over the same files.

## Migration Plan

`fleet metrics` → `metrics`. `fleet logs` → `logs`. `remote metrics` →
`metrics --env <name>`. `remote logs` → `logs --env <name>`. `remote status` →
`status --env <name>`. Each old spelling fails naming its replacement. Nothing
is persisted or transmitted differently; rollback is a revert.
