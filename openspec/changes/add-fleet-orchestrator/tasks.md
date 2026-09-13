## 1. Fleet file: tags and concurrency limits

- [x] 1.1 Add node `tags` (key/value map, non-empty strings, duplicate key refused) to the fleet file parse and validation in `internal/fleet/config.go`; verify with unit tests covering a tagged node, a duplicate-key refusal naming the node and key, and an untagged file behaving as before
- [x] 1.2 Add the top-level `concurrency` section (`total`, per-tag limits keyed by `key=value`) with positive-integer validation naming the offending entry; verify with unit tests covering both limits, a zero/negative refusal, a limit on a tag no node carries, and no section unchanged
- [x] 1.3 Expose both on the parsed `Config` the way `Wakes()` and `GatewaySection()` do, and keep a file with neither parsing and behaving exactly as today; verify `go test ./internal/fleet/...` passes and existing fleet tests are untouched

## 2. Gateway: the topology endpoint

- [x] 2.1 Add the topology reply type and handler to `internal/gateway`: each node's name, kind, tags, state, served model and name, readiness, last-active, and for a stopped node the model a request would start it with (only where its source describes one and the fleet wakes), plus the file's wake policy, preference, and concurrency limits, each absent where undeclared; a node that does not answer is reported in place, not an error; verify with unit tests over the fake fleet, including the dead-node and absent-settings cases from the spec scenarios
- [x] 2.2 Serve the reply on the gateway's read path behind the caller authentication, built from the same cached fan-out and source resolution the model listing uses; verify with a unit test that a caller with the token gets the topology and one without is refused as on the gateway's other paths
- [x] 2.3 Update the gateway's `Paths the gateway does not serve` behaviour tests so the new path is served and nothing else changes; verify `go test ./internal/gateway/...` passes

## 3. Orchestrator core

- [x] 3.1 Define the work item (unique id, instructions, working directory, optional tags as `key=value` pairs, optional priority) and its file source: parse, validate (duplicate id and missing fields naming the item), and report file-order; verify with unit tests covering a minimal item, duplicate-id refusal, and an unparseable file
- [x] 3.2 Write the pure matching and admission logic: an item matches a node only where every carried tag is the node's; running-and-answered nodes rank before a node to be started, then by the file's preference; a stopped node is eligible only where the fleet wakes; an item is admitted only while the total and every carried tag's limit have room, counting the in-flight set; verify with unit tests covering the spec's match and admit scenarios, including no-tags-matches-any and nothing-matched-waits
- [x] 3.3 Write the state the process keeps beside the items file (`<file>.state.json` plus per-item logs under `<file>.logs/`): item states across a restart, an item left running recorded failed naming the interruption, a clean interrupt re-queueing its items, and a second orchestrator for the same file refused; verify with unit tests over a temp-directory state
- [x] 3.4 Write the dispatch: synthesize the selection from the gateway address and the chosen node's model name (served name where reported, else the model id), apply the provider into the harness config under a lock, and run the active harness's non-interactive single-task form in the item's directory with output captured per item; a harness without a single-task form and a missing directory fail the launch naming the cause; verify with unit tests against a stub harness executable
- [x] 3.5 Write the loop: tick and re-read the gateway's topology (a gateway that stops answering ends the command naming it), re-read the items file on change, admit per the pure logic, reap finished children, and keep one orchestrator per items file; verify with a unit test driving the loop against a fake topology and a stub agent, asserting admission order, limits, and ends

## 4. The command

- [x] 4.1 Add `spinloop orchestrator` as a top-level command: required gateway flag, `--items` defaulting to `./work.yaml`, the token variable flag defaulting to `OPENAI_API_KEY`, the long-running foreground lifecycle; an unreachable or refusing gateway fails before any item is worked; verify with command tests and that `spinloop orchestrator --help` reads per the cli-ux conventions
- [x] 4.2 Wire the command into the command tree's help and the `__complete` engine (command name, flags, and the items-file path completion where the engine offers one), keeping `__complete` error-free and silent on stderr; verify with the existing completion tests extended for the new command

## 5. Integration tests (end to end, in process)

- [x] 5.1 Add an integration test that stands up the real gateway handler on a loopback address over a fake fleet and runs the real orchestrator loop against it with a stub agent: items flow backlog → running → done, a tag limit and the total each hold the line, and a finished item frees its slot; verify the test asserts in-flight never exceeds a limit at any point
- [x] 5.2 Extend the integration test to the fleet-shape scenarios: an item takes only a node carrying all its tags, a running node is offered before a stopped one, a stopped node is used only where the fleet wakes, and an item nothing matches waits until a node appears in the topology; verify each as a subtest
- [x] 5.3 Extend the integration test to the lifecycle: a crash between starts records a running item failed without re-running it, a clean interrupt re-queues and the next start picks the items up, and a missing working directory fails one item while the rest run; verify by restarting the loop in the test
- [x] 5.4 Run the full suite and keep total coverage at or above 80%: `go test ./... -cover`; verify the total reports >= 80% and no package with new code sits meaningfully below its neighbours

## 6. Docker end-to-end and docs

- [x] 6.1 Add an orchestrator scenario to `examples/gateway-docker/run-tests.sh` beside the existing gateway scenarios: a gateway plus a fleet plus an orchestrator working a small items file with a stub agent, asserting the concurrency limits held and the items ended done; verify the script's new steps follow the file's existing structure and `bash -n` accepts it
- [x] 6.2 Document the new surface in `docs/`: the orchestrator command reference (its flags, the items file's shape, the state and logs it keeps, what it deliberately does not do) and the fleet file's `tags` and `concurrency` sections in the fleet reference, plus the topology endpoint in the gateway reference; verify each doc's claims against the implemented behaviour
- [x] 6.3 Note the new layout entry in `AGENTS.md` the way `internal/gateway` is noted, and the orchestrator in the command list at the top of the file; verify the file still reads as an orientation map

## 7. Final verification

- [x] 7.1 Run `go build ./...`, `go vet ./...`, `gofmt -l .` (clean), and `go test ./... -cover` (>= 80%); verify all four pass
- [x] 7.2 Walk the spec scenarios of `fleet-orchestrator`, `fleet-config`, and `fleet-gateway` against the tests written here; verify every scenario has a named test or a subtest covering it
