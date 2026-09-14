# Tasks: add the orchestrator's work list API

## 1. The Store interface and the shared core

- [x] 1.1 Define the `Store` interface in `internal/orchestrator` — the per-item records (load/save), each item's log path, the lock's lifecycle (open, refuse a second orchestrator naming the holder, take over a stale lock, release), and the recovery a restart owes (an item left running recorded failed, naming the interruption) — and move the file-backed store from `state.go` behind it as the default implementation, its paths and shapes unchanged; verify the existing state and lock tests pass against the interface, and `go test ./internal/orchestrator/` is green
- [x] 1.2 Rebuild the run loop on the `Store` interface and a shared core — the records, the in-flight set, and the items view under one mutex — with the loop's pass and each API operation as functions on the same state, the items view refreshing when the loop re-reads a changed file and when the API writes one; verify the existing loop, dispatch, and integration tests pass unmodified (admission order, limits, ends) and a new test drives the core from two goroutines — the loop and a handler-shaped caller — under `-race` with no data race reported

## 2. The work list API handler

- [x] 2.1 Write the handler in `internal/orchestrator`: `GET /v1/items` (every item with its record, the file's order, backlog where there is no record), `GET /v1/items/{id}/log` (the kept output, an item with none answered as having none), `POST /v1/items`, `DELETE /v1/items/{id}`, `POST /v1/items/{id}/abort`, `/health`, and a `404` naming the paths it serves for every other method or path; verify with unit tests over the core with a fake store and no agent, one per path and one for the `404`
- [x] 2.2 Put every path behind the bearer the gateway's way — a constant-time compare, `/health` included, and a tokenless handler on loopback asking nothing — and verify with tests that a caller with the token is served on every path, a caller with no token or the wrong one is refused on every path, and a tokenless handler serves
- [x] 2.3 Implement the mutations on the core: add — the items file's validation, an id the file already carries refused naming it, an id the state records done or failed refused naming the record, the file a valid items file after the write; remove — the item, its record, and its kept output, a running item refused naming it and saying to abort it first, an id the file does not carry refused naming it; abort — the item's agent stopped the way a clean interrupt stops it and the item back in the backlog, an item that is not running refused naming the item and its state; verify with a test for every acceptance and every refusal, each asserting the state and the items file after

## 3. The command's server

- [x] 3.1 Give `spinloop orchestrator` the gateway's server flags — `--listen` defaulting to `:4010`, `-l/--loopback` binding `127.0.0.1:4010`, `--api-token`, and `--api-token-file`, the token resolved the daemon's way — with a non-loopback bind and no resolvable token refused before serving, naming the address and the ways a token may be supplied, and an explicit address with `--loopback` failing before serving, naming both; verify with command tests: loopback serves with no token, a non-loopback bind without a token is refused before serving, the conflict names both flags, and the token from a file and from the environment are honoured
- [x] 3.2 Stand the server up before the run's first pass, the way the gateway has its handler in before a signal can arrive, take it down with the clean interrupt after the agents are stopped and their items back in the backlog, and print the address the way the gateway prints its own; verify with a command test that a request is answered while the run works, a clean interrupt leaves the state saved and the server down, and the startup line names the address
- [x] 3.3 Keep the API's mutations and the loop on the one core, so an add or remove the API accepts reaches the run on its next pass and an abort stops the child the loop would reap; verify with a test running the loop against a fake topology and a stub agent while a client adds, aborts, and removes items — the added item flows backlog to done, the aborted item returns to the backlog and is admitted again, and the removed item's record and log are gone

## 4. The e2e

- [x] 4.1 Add an orchestrator API scenario to `examples/gateway-docker/run-tests.sh` beside the orchestrator scenario: the work list shows the items as they flow, an add over the API is worked to done, a remove takes an item with its record and its output, and an abort returns a running item to the backlog; verify `bash -n` accepts the script

## 5. Docs and the final pass

- [x] 5.1 Document the API in `docs/`: the orchestrator command reference's new flags, the API's paths with what each does and refuses, the loopback affordance, and the two tokens — the fleet gateway's and the API's, and how they differ; verify each doc's claims against the implemented behaviour
- [x] 5.2 Walk the spec's scenarios — "The orchestrator command"'s new ones and every "Serving the work list API" scenario — against the tests, and run `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test ./... -cover`; verify every scenario has a named test or subtest, the tree is clean, and the total coverage stays at or above 80%
