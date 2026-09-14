## Why

The gateway's on-demand wake only ever starts an engine on a daemon node.
A deployed-but-stopped remote environment already knows what to serve — the
same information a status read already gets back from its control plane —
but the gateway refuses to start it and never advertises it as reachable, so
a request that could be served by waking a stopped remote environment fails
instead, even though the fleet file allows waking and the node is one
`start` call away from serving it.

## What Changes

- `remoteNode.StartWith` boots a deployed-but-stopped environment's instance
  instead of refusing outright. An undeployed environment — nothing stored to
  serve — is still refused, unchanged.
- The gateway's wakeable-model computation (`/v1/models`, the topology
  endpoint, and the wake candidate search) stops skipping remote nodes. A
  stopped remote node's wakeable model is read from its last status reply
  (served name first, then model id) — the same fact the control plane
  already reports for a deployed environment — rather than resolved from a
  local Spinloop source the way a daemon node's is.
- A node entry gains an optional per-node `wake` override (`on`/`off`). When
  set, it decides whether that node may be woken regardless of the fleet's
  own `wake` setting; when unset, the fleet's setting applies as it does
  today. This lets an operator leave a fleet's `wake` on for its daemon nodes
  while opting a remote node in or out individually — starting a remote
  instance costs money, so its opt-in should not be implied by the fleet's
  blanket setting alone.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-gateway`: the models list, the topology endpoint, and the wake
  candidate search now consider a deployed-but-stopped remote node wakeable,
  sourcing its wakeable model from its own status reply instead of a
  Spinloop source.
- `remote-node`: waking a remote environment is no longer refused
  unconditionally — a deployed-but-stopped environment is started; an
  undeployed one is still refused.
- `fleet-config`: a node entry may declare its own `wake` setting, overriding
  the fleet-wide one for that node alone.

## Impact

- `internal/fleet/remote_node.go` (`StartWith`), `internal/fleet/wake.go`
  (candidate resolution), `internal/fleet/config.go` (`NodeConfig`, per-node
  wake resolution), `internal/fleet/node.go`.
- `internal/gateway/gateway.go` (`wakeableModels`, `handleModels`,
  `handleTopology`, `wakeFor`, `refuseWake`).
- `cmd/spinloop/gateway.go` (the `ConfigFor` the gateway wires up).
- No change to the orchestrator's admission (#188) or to undeployed remote
  environments, which remain out of scope per the issue.
