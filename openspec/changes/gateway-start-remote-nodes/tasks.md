## 1. Per-node wake override

- [x] 1.1 Add `NodeConfig.WakePolicy WakePolicy` (`yaml:"wake,omitempty"`) in
      `internal/fleet/config.go`, validated at parse time the same way the
      fleet-wide `wake` value is (`ParseWakePolicy`, failing on anything but
      `on`/`off`/absent), and verify with a parse test covering a valid and
      an invalid per-node value.
- [x] 1.2 Add `Config.NodeWakes(entry NodeConfig) bool`: the node's own
      `WakePolicy` when it names one, else `c.Wakes()`. Verify with unit
      tests for all four combinations (fleet on/off crossed with node
      unset/on/off).
- [x] 1.3 In `internal/fleet/wake.go`, skip a candidate in `Wake`'s own loop
      when `c.NodeWakes(cand.entry)` is false, refusing it with a
      "waking is disabled for this node" reason the same way a
      config-mismatched candidate is refused — `wakeable()`'s ordering and
      `WouldWake` stay policy-agnostic, since a caller (`fleet route`'s and
      the gateway's pre-emptive refusal) still needs to name the node a
      wake-off setting refused. Add `Config.AnyNodeWakes() bool` for that
      pre-emptive check, and swap it in wherever `cfg.Wakes()` gated a
      pre-`Wake` refusal, including `cmd/spinloop/route.go`. Verify with a
      `Wake` test where a fleet that wakes has one node with `wake: off`
      that must not be chosen, and one where a fleet that does not wake has
      one node with `wake: on` that must be chosen.

## 2. Remote node starts on wake

- [x] 2.1 Replace `remoteNode.StartWith` in `internal/fleet/remote_node.go`
      with a delegation to `StartWithProgress` (ignoring `dc` and
      `engineKey`, as `Start` already does), removing the unconditional
      refusal. Verify with a test that `StartWith` boots the same way
      `Start` does against a fake control plane.
- [x] 2.2 Have `StartWith` read a fresh status first and refuse immediately,
      naming `spinloop remote deploy`, when it reports nothing served —
      `remote.Start`'s own retry loop treats an undeployed environment's
      `503` the same as a capacity wait, so leaving it to that call would
      hold the request for the whole wake timeout instead of failing fast.
      Verify with a test that `StartWith` against a fake control plane
      reporting no served model refuses immediately without calling the
      boot endpoint.

## 3. Gateway: resolving a remote node's wakeable config

- [x] 3.1 In `internal/gateway/gateway.go`, add a resolver that builds an
      `inference.DeployConfig{ModelID, ServedModelName}` for a `KindRemote`
      entry from its `NodeResult.Status` (`Model`, `ServedName`) within a
      given `results []fleet.NodeResult`, erroring when there is no OK
      result for that node or when it reports nothing being served (nothing
      deployed) — mirroring the error a daemon node with no resolvable
      source already produces. Verify with a unit test covering: a deployed
      status resolves; an undeployed (empty Model/ServedName) status errors
      naming `spinloop remote deploy`; a missing/not-OK result errors.
- [x] 3.2 Combine that resolver with `h.cfgFor` into one `fleet.ConfigFor`
      keyed by `entry.Kind`, and use the combined resolver everywhere
      `h.cfgFor` is currently passed to `matchingConfigFor`, `Wake`, and the
      wakeable-models loop. Verify by confirming existing daemon-path tests
      for `wakeFor`/`refuseWake`/`wakeableModels` still pass unchanged.

## 4. Gateway: advertising and waking a remote node

- [x] 4.1 Change `wakeableModels()` to accept `results []fleet.NodeResult`,
      drop the `entry.Kind != fleet.KindDaemon` skip, resolve every node's
      wakeable model through the combined resolver from 3.2, and skip a
      node when `!h.cfg.NodeWakes(entry)`. Update its two call sites
      (`handleModels`, `handleTopology`) to pass the `results` they already
      hold. Verify with a test that a deployed, stopped remote node's model
      appears in `/v1/models` and in the topology's `wakeableModel`, and
      that an undeployed one, and a wake-disabled one, do not.
- [x] 4.2 Change `wakeFor`/`refuseWake` to check `h.cfg.NodeWakes(entry)` per
      candidate node instead of the single `h.cfg.Wakes()` gate, and to use
      the combined resolver. Verify with a test that a request for a model
      only a deployed, stopped remote node serves is held and answered once
      that node's `StartWith` reports it running (using a fake remote
      control plane, the way existing remote-node gateway tests already
      fake one).
- [x] 4.3 Verify with a test that a request for a model only an undeployed
      remote node's name could match still fails, naming the deployment
      path, and that a request for a model only a wake-disabled remote node
      serves fails naming that it is not woken — both without starting
      anything.

## 5. Specs and docs

- [x] 5.1 Run `openspec validate --strict gateway-start-remote-nodes` (or
      the store-scoped form if applicable) and fix any reported issues in
      the delta specs.
- [ ] 5.2 Update `docs/` and `README.md` fleet/gateway documentation, if any
      describes the current "remote nodes are never woken" behaviour or the
      fleet-wide-only `wake` setting, to reflect the per-node override and
      remote wake support (handled by `/docs-update` after implementation).

## 6. Full verification

- [x] 6.1 Run `go test ./... -cover` and confirm coverage stays at or above
      80%, with the new/changed packages (`internal/fleet`,
      `internal/gateway`) individually meeting it. All packages pass;
      `internal/fleet` 89.2%, `internal/gateway` 93.8%, `cmd/spinloop` 91.3%.
- [x] 6.2 Run `gofmt -l .` and confirm it reports nothing.
