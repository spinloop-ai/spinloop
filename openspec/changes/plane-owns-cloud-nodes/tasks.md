# Tasks

Groups 1–2 ship to the control plane and are green with an unmodified CLI.
Groups 3–8 ship in the CLI. Nothing before group 3 changes observable CLI
behaviour.

## 1. Control plane: inventory and ownership

- [x] 1.1 Store `deployedBy` and `deployedAt` alongside the deploy config in `/cloud-vm-llm/<env>/deploy-config`, taken from the invoking principal, and verify with a `remote/` unit test that a deploy records both and a redeploy replaces them
- [x] 1.2 Add an inventory branch to the control API that enumerates every deployed node with name, deployer, deploy time, runner, model id, served name, address, instance state and instance type, and verify with `remote/` tests covering a plane with several nodes and one with none
- [x] 1.3 Grant the inventory branch the SSM and EC2 reads it needs through its own role and verify the deployed stack template asserts no new permission on the `cloud-vm-llm-remote-cli` policy beyond what it already holds
- [x] 1.4 Confirm the control-plane version stamp rides the package version, `pnpm test` in `remote/` passes, and the CLI still names the re-bootstrap repair for planes predating an endpoint (version-mismatch warning tests plus the schedule test over a plane with no schedule endpoint)
- [x] 1.5 Document the inventory reply in the control-API notes beside `remote/lambda` and verify the reply's field names appear in it

## 2. Control plane: undeploy

- [x] 2.1 Implement undeploy for one named node: terminate the instance, release the Elastic IP, delete the security group, delete the key secret, remove its schedules, delete the deploy-config parameter, and verify with `remote/` tests asserting each resource is gone and the reply lists what was released
- [x] 2.2 Make undeploy idempotent and scoped: an unknown node answers a clear not-deployed reply, and sibling nodes' resources are untouched, verified by tests for both cases
- [x] 2.3 Verify a name undeployed and then deployed again comes up as new — new security group, new key, ingress required — with a test asserting the fresh deploy does not inherit the removed config
- [x] 2.4 Confirm no shared resource (Lambdas, roles, VPC, weights bucket) is reachable from the undeploy path and verify by reviewing the granted resources in the stack test snapshot

## 3. CLI: resolve the plane, remember it in memory

- [x] 3.1 Add a plane resolver in `internal/cloud` that resolves region (explicit, then AWS region resolution, then profile) and reads the stack outputs, reusing `DiscoverControlPlane`, and verify with unit tests over a stubbed CloudFormation client covering each region source and the no-region failure
- [x] 3.2 Memoise the resolved plane per process with no disk write, and verify a test asserting one discovery call serves four sequential node reads and that a dashboard-style loop still issues only one
- [x] 3.3 Revalidate once — re-resolve and retry a single call — when a control call is rejected as unauthorised or fails with an unreachable host, and verify with a test that a changed host heals on the second attempt and that a persistent rejection still fails
- [x] 3.4 Make `SPINLOOP_CLOUD_*` overrides apply on top of the resolved plane value-by-value and accept a complete set with no stack at all, and verify both with tests
- [x] 3.5 Fail loudly naming stack, account and region when no plane resolves, and verify a test asserts the message contains all three plus `spinloop cloud bootstrap`
- [x] 3.6 Replace `cloud.LoadEnvironment` with plane-plus-name resolution at every call site and delete the `Config` fields that held stored URLs, verifying `go build ./...` and `go vet ./...` succeed and the affected tests pass

## 4. CLI: remove the registry

- [x] 4.1 Delete `internal/cloud/environments.go` and its tests, removing `EnvDir`, `EnvConfigPath`, `SaveEnvironment`, `ListEnvironments` and `IsEnvName`'s registry callers, and verify `go build ./...` reports every remaining caller so none are left behind
- [x] 4.2 Rework `runDeploy` to ask the plane whether the node exists rather than `os.Stat`-ing a config file, and verify with tests that a colleague's node is detected from a clean machine and that nothing is written beneath the config directory on deploy
- [x] 4.3 Make the overwrite warning name the deployer, deploy time and served model from the inventory, and verify with a test asserting all three appear
- [x] 4.4 Refuse to redeploy over an existing node when no `--allowed-cidr` is stated, keeping the detected-`/32` default for a new one, and verify with tests that an existing node's ingress is untouched on refusal and that a stated CIDR is applied
- [x] 4.5 Remove the `registered: <path>` line from the deploy report and print the plane it deployed to instead, and verify the deploy output test
- [x] 4.6 Rewrite the registry-backed tests in `cmd/spinloop` and `internal/fleet` against a plane stub, and verify `go test ./cmd/spinloop/... ./internal/fleet/... ./internal/cloud/...` passes with no test reading a `clouds/` directory
- [x] 4.7 Update every "is not registered" failure and the `--node` flag help text to name the plane and the deploy command, and verify by grepping for `not registered` returning nothing outside this change

## 5. CLI: `cloud ls`, `cloud undeploy`

- [x] 5.1 Point `spinloop cloud ls` at the inventory, listing name, deployer, served model, address and state, and verify with tests covering a populated plane, an empty plane (says so), and an unreachable plane (fails naming where it looked)
- [x] 5.2 Add `spinloop cloud undeploy --node <name>` with `--dry-run` and explicit consent, refusing to act when no node is named, and verify with tests for each of the three paths plus the destroy report's contents
- [x] 5.3 Add the undeploy examples and the `ls` columns to the cloud pages under `docs/`, and verify `scripts/docs-serve.sh` renders the changed pages
- [x] 5.4 Note in the release notes and docs that `~/.config/spinloop/clouds/` is no longer read or written and may be deleted, and verify the release note text exists

## 6. CLI: fleet nodes, membership and gateway

- [x] 6.1 Resolve `kind: cloud` nodes and the `--node` fleet of one through the plane, keeping the fleet file's schema byte-identical, and verify with tests that a cloud node with no local state resolves and that a node the plane lacks is a per-node row naming the plane
- [x] 6.2 Keep fleet membership driven solely by the file — never by the inventory — and verify with tests that a plane node absent from the file is not listed, routed to, or woken
- [x] 6.3 Settle cloud-node existence in the gateway's candidate choice from the inventory rather than a failing stats read, and verify with tests that an unlisted-but-deployed node is a candidate and one the plane lacks is refused with the deployment path named
- [x] 6.4 Rework `internal/fleet.ForEnvironment` and `cmd/spinloop/target.go` so no code path consults a local record, and verify by grepping for registry references under `internal/fleet` and `cmd/spinloop` and finding none
- [x] 6.5 Update `docs/` fleet pages to state that a committed fleet file with cloud nodes works for anyone with credentials for the plane, and verify the rendered page

## 7. CLI: completion and error surfaces

- [x] 7.1 Source `--node` completion from the fleet file in force followed by the plane's nodes, and verify with a completion test that both sets appear in that order
- [x] 7.2 Degrade every unresolvable completion source to an empty offer with a zero exit and nothing on stderr, and verify with a test that drives completion with an unreachable plane
- [x] 7.3 Check the new failure copy against the `cli-ux` conventions (errors name the fix, decoration only on a terminal, nothing extra on a consumable stdout) for `ls`, `undeploy` and deploy refusals, and verify by reading the recorded conventions and running the commands' tests

## 8. Integration checks

- [x] 8.1 Run `go test ./... -cover` and confirm total coverage is at least 80% and every suite passes
- [x] 8.2 Run `pnpm test` and `pnpm run synth` checks in `remote/` and confirm the stack snapshot changes only where groups 1–2 added resources
- [x] 8.3 Run `gofmt -l .`, `go vet ./...` and `scripts/check-no-cloud-identifiers.sh` and confirm all three report nothing
- [x] 8.4 Run the `examples/fleet-docker` fleet integration path CI runs per PR and confirm it still passes, since it exercises mixed-node fan-out and the gateway
- [ ] 8.5 Verify end to end against a real plane: `cloud bootstrap`-less machine → `cloud ls` lists the plane, `--node` resolves a node deployed elsewhere, `cloud undeploy --dry-run` lists its resources, and confirm nothing new appears under `~/.config/spinloop/`
- [ ] 8.6 At archive time, reword the surviving "environment" scenario titles inside the synced `openspec/specs/` requirements (kept verbatim so the delta's MODIFIED blocks match) to the node wording, and confirm `openspec validate --specs` passes
