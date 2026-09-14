## Why

`internal/daemon` imports `internal/remote` for one type and one helper:
`remote.DeployConfig`, the description of what an engine should serve, and
`remote.ConfigHome`, which only forwards to `internal/config.Dir`. The daemon
supervises a local engine and has no cloud in it, so the dependency is
backwards: the package that knows nothing about AWS depends on the package that
is nothing but AWS, for a vocabulary the two share.

It is also in the way. Later phases of the command work route the cloud read
commands through the `fleet.Node` contract so one verb serves every target; a
node kind that cannot be described without importing the cloud control plane
makes that harder than it needs to be. Fixing the layering first keeps that
work about commands rather than about imports.

## What Changes

- A new leaf package holds `DeployConfig` — the runner-neutral statement of
  what to serve — which `internal/daemon` and `internal/remote` both import.
  Neither imports the other for it, and the new package imports nothing of
  ours.
- `daemon.StateDir` takes its directory from `internal/config.Dir` directly
  rather than through `remote.ConfigHome`. The resolved path is unchanged:
  `ConfigHome` already forwards to `config.Dir` and nothing else.
- `internal/daemon` no longer imports `internal/remote` at all.
- The JSON field names and shape of `DeployConfig` are untouched, so the
  daemon's control API, the control plane's stored configs, and every message
  already on the wire are unaffected.

No user-visible behaviour changes: no command, flag, output, file format or
HTTP contract moves. This is a pure refactor.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

(None — no spec-level behaviour changes. The change sets `skip_specs: true`.)

## Impact

- New package holding `DeployConfig`, moved verbatim from
  `internal/remote/remote.go`.
- `internal/daemon` (`daemon.go`, `api.go`): imports the new package for the
  type and `internal/config` for the directory; drops `internal/remote`.
- `internal/remote`: keeps `Deploy` and the rest, importing the type from its
  new home. `IsInstanceType` stays here — see design.
- `internal/fleet` (`node.go`, `client.go`, `wake.go`, `remote_node.go`),
  `internal/gateway`, and `cmd/spinloop` (`remote.go`, `serve_daemon.go`,
  `gateway.go`): the type is referenced under its new package name. 34
  non-test references across 10 files, plus their tests.
- `AGENTS.md` and `docs/internals.md`: the package list and the layering note.
- No change to `remote/` (the TypeScript control plane), `docs/openapi.yaml`,
  or any on-disk format.
