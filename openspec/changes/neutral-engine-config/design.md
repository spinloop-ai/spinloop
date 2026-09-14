## Context

See proposal.md — Why. The implementation-relevant current state:

- `remote.DeployConfig` (`internal/remote/remote.go:241`) is the runner-neutral
  description of what to serve. It is a plain struct with JSON tags, no
  methods, and no dependency on anything in `internal/remote` around it.
- `internal/daemon` references it 8 times (`daemon.go`, `api.go`): the stored
  config, the `BuildArgv`/`EngineKeyArgs`/`ValidateConfig` hooks the CLI
  injects, and the start request's body.
- `remote.ConfigHome` is `return config.Dir()` and nothing else
  (`internal/remote/environments.go:17-19`). `daemon.StateDir` is its only
  caller outside `internal/remote`.
- `internal/config` and `internal/metrics` are already leaves — neither
  imports another `internal/` package — so a third leaf creates no cycle.
- Outside the daemon, `DeployConfig` is referenced by `internal/fleet`
  (`node.go`, `client.go`, `wake.go`, `remote_node.go`), `internal/gateway`,
  and `cmd/spinloop`. Those packages import `internal/remote` for other
  reasons anyway, so for them this is a rename, not a dependency change.

## Goals / Non-Goals

**Goals:**

- `internal/daemon` imports no cloud package.
- The shared "what to serve" vocabulary lives somewhere neither side owns.
- The move is mechanical and reviewable: a type relocated, imports adjusted,
  no logic edited.

**Non-Goals:**

- No change to what `DeployConfig` holds, how it serialises, or what validates
  it. A field added or renamed here would hide a wire change inside a refactor.
- No capability interfaces on `fleet.Node` (`Coster`, `SourceLogger`, and the
  rest). They have no consumer until the read verbs collapse, and the pattern
  is already established by `ProgressStarter` and `Keeper`, so deferring them
  costs nothing.
- No move of `daemon.StatusResponse` — see D3.
- No renaming of the `internal/remote` package, the `remotes/` registry
  directory, or the `SPINLOOP_REMOTE_*` variables. The `remote` → `cloud`
  rename is its own change.

## Decisions

**D1: The new package is `internal/engine`, a leaf holding `DeployConfig`.**

The name says what the type describes — what an engine serves — rather than
which caller wants it. It imports only the standard library, so every current
and future node kind can depend on it.

Alternatives: putting the type in `internal/spinloop` (rejected — that package
is the Spinloop *file* grammar, a pure parse leaf, and a deploy config is not a
file format); putting it in `internal/config` (rejected — that is spinloop's own
config file, a different concern that happens to share a word); leaving it in
`internal/remote` and having `daemon` keep the import (rejected — that is the
problem being fixed).

**D2: `IsInstanceType` stays in `internal/remote`.**

It validates `DeployConfig.InstanceType`, so cohesion argues for moving it with
the type. Against that: it encodes the shape of an *EC2* instance type, only
cloud deploys and `kind: remote` fleet nodes call it, and `internal/daemon`
never does. Moving it would pull AWS vocabulary into a package whose whole
point is not having any.

A field in the neutral package validated from the cloud package is the right
split: the cloud is the only thing that acts on that field. Alternative:
move it and accept the EC2 knowledge in `internal/engine` (rejected — it would
make the next AWS-shaped helper look like it belongs there too).

**D3: `daemon.StatusResponse` does not move.**

It is tempting to pair it with `DeployConfig` as the other half of a shared
vocabulary, but the two are not alike. `DeployConfig` is a *request* both sides
accept — the daemon is handed one, the control plane stores one. `StatusResponse`
is the daemon control API's *reply*, published in `docs/openapi.yaml` and
governed by `daemon-api-contract`; the cloud path deliberately translates into
it (`statusFromRemote`) so that one fan-out and one set of renderers cover both
kinds. That translation is the design working, not a layering fault.

Moving it would rename a published contract for no behavioural gain and churn
the OpenAPI document. Alternative: move both now for symmetry (rejected — it
widens a mechanical change into one touching the API contract, and the
symmetry is superficial).

**D4: `daemon.StateDir` calls `config.Dir` directly.**

`remote.ConfigHome` forwards to it and does nothing else, so this removes the
second of the daemon's two reasons to import the cloud package at no
behavioural cost. `remote.ConfigHome` stays for `internal/remote`'s own callers.

Alternative: move `ConfigHome` to the new package too (rejected — it is about
spinloop's config directory, which `internal/config` already owns; a third
name for it would be worse than the one forward it replaces).

**D5: The move is verbatim, in one commit, with no behavioural edits.**

The type, its doc comment and its field comments move unchanged; only the
package clause and the references change. Keeping the diff free of logic edits
is what makes a 10-file change reviewable, and it means a `git log --follow`
on the type still reads.

## Risks / Trade-offs

- [A silently changed JSON tag would break the daemon API and the control
  plane's stored configs at once] → the struct is moved verbatim, and the
  existing `internal/daemon` OpenAPI test plus the control-plane round-trip
  tests cover the wire shape; no field is touched in this change.
- [A large mechanical diff hides a real edit] → the change adds no logic, so
  review is "is anything here not a package rename?"; the build and the full
  suite must pass unchanged, with no test edited except for its import block.
- [Another change lands on the same files and conflicts] → the touched
  packages are stable and the diff is import-shaped, so conflicts resolve
  mechanically; the change is small enough to land quickly rather than sit.
- [`internal/engine` becomes a dumping ground for anything two packages share]
  → D1 and D2 fix the admission rule: it holds what describes an engine's
  workload and imports nothing of ours. Anything cloud-shaped stays in
  `internal/remote`.

## Migration Plan

None. Nothing is persisted, transmitted or configured differently, so there is
no data to migrate and no version skew: a binary built before this change and
one built after speak the same protocol. Rollback is a revert of the commit.
