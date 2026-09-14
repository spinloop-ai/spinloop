# Add the orchestrator's work list API

## Why

The orchestrator's work list — the items, each one's record, and the output
its agent kept — is reachable only by reading files on the machine the
orchestrator runs on, and nothing outside the process can steer it. The state
itself is a concrete file-backed struct bound into the run loop, so a second
reader cannot share it and every client would re-derive the file layout and
race the orchestrator's lock. The `work` command family (#206-#209) and any
other client need a stable front door to the work list instead.

## What Changes

- The orchestrator's state store becomes pluggable: a `Store` interface — the
  per-item records, the per-item logs, and the lock that keeps a second
  orchestrator off the items file — with the current file-backed store as the
  default implementation. The run loop and the new API both work through the
  interface. No observable change: the state still lives beside the items
  file, in the same shape, and the lock still refuses a second orchestrator.
- The orchestrator serves the work list over HTTP, the same pattern as
  `spinloop gateway`: a `--listen` address with a fixed default, a
  `--loopback` / `-l` affordance that binds loopback on the default port and
  needs no token, `--api-token` and `--api-token-file` resolved the way the
  daemon's token is, a non-loopback bind refused without a token, and a
  bearer token on every path. A path the API does not serve is answered
  `404` naming the paths it does.
- The API answers queries and mutations on the work list: every item with its
  record (backlog, running, done, or failed — the node and the times where
  they apply), one item's kept agent output, and the mutations add an item,
  remove an item, and abort a running item — each with the validation and
  refusal the items file and the run give today, so a client cannot put the
  work list in a state the orchestrator itself would not accept.
- The `work` command family (#206, #207, #208, #209) builds on this API
  rather than on the files directly.

## Capabilities

### New Capabilities

(none — the API belongs to the orchestrator capability)

### Modified Capabilities

- `fleet-orchestrator`: "The orchestrator command" gains the server's
  address and token flags and the loopback affordance; a new requirement
  covers the work list API — what it serves, how callers authenticate, and
  what each query and mutation does and refuses.

## Impact

- `internal/orchestrator` — the `Store` interface and the file store
  implementation (today's `state.go`), the shared work list the loop and the
  API both act on, and the HTTP handler with the gateway-style listen,
  token, and authentication rules.
- `cmd/spinloop/orchestrator.go` — the new flags and the server's lifecycle
  beside the loop's.
- `openspec/specs/fleet-orchestrator/spec.md`, the orchestrator's command
  reference in `docs/`, and the docker e2e beside the orchestrator scenario.
