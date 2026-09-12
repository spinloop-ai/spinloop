# Tasks

## 1. The fleet file's gateway section

- [x] 1.1 Parse an optional top-level `gateway` section in the fleet file — a `url` (required, carrying a scheme; a section without one refused naming the field) and a `tokenEnv` naming the token's variable, absent one defaulting to `OPENAI_API_KEY` — with a `fleet.Config` accessor for the section, verified by config tests (a section with both fields, a section with only a url, a section with no url refused naming the field, and no section leaving the file unchanged)

## 2. Routing at a file's gateway

- [x] 2.1 Route a launch whose effective fleet file names a gateway at that gateway: the section's address written as the applied provider's base URL with the OpenAI-compatible prefix and as `OPENAI_BASE_URL`, the token resolved from the section's variable through the client's key chain (an already-set variable winning, a variable set nowhere failing before anything is written and naming it), no node contacted and none woken, and a pinned `BASEURL` still winning and saying so, verified by launch tests for each
- [x] 2.2 Make `spinloop fleet route` answer a Spinloop whose fleet file names a gateway by naming the address the launch will be given, querying no node and starting nothing, verified by a command test

## 3. The command

- [x] 3.1 Add `spinloop fleet harness` beside the other fleet subcommands: a Spinloop taken the way `spinloop harness` takes one (`-O`, a leading alias or path, or the `Spinloop` beside it), a fleet file from `--fleet`/`-f` defaulting to the `fleet.yaml` beside it, and the launch's steering flags (`--node`, `--prefer`, `--no-wake`, `--wake-timeout`); with no `-f` a Spinloop's `FLEET` (file or endpoint) is used, and an explicit `-f` wins over it; it runs the launch path's routing — the gateway where the file names one, node selection and wake otherwise — announces the choice on stderr, applies the Spinloop, and launches the harness; a command with no Spinloop fails before launching, saying a launch needs a Spinloop to know which model to route, verified by command tests (the gateway is used when the file names one, a node is routed to when it names none, the command's file wins over the instruction, and no Spinloop fails) and by the completion dispatch-coverage scan passing with the new subcommand

## 4. Example and documentation

- [x] 4.1 Move `examples/gateway-docker/` onto the command: the example's fleet file gains a `gateway` section, the client Spinloop loses its `FLEET` URL, and `run-tests.sh` asserts the client is configured through `spinloop fleet harness` and reaches the fleet through the section's address, verified by `./run-tests.sh` passing
- [x] 4.2 Document the change where the rest of it is documented: the `gateway` section in the fleet file documentation, and `docs/commands/gateway.md` naming `spinloop fleet harness` as the way to point a harness at a fleet, verified by the docs links resolving and the new pages reading against the implemented flags

## 5. Final verification

- [x] 5.1 Run the full gate: `go test ./... -cover` at or above the 80% total, `go vet ./...`, `gofmt -l .` clean, the control plane's `pnpm build` and `pnpm test`, `openspec validate add-fleet-harness`, and `examples/gateway-docker/run-tests.sh`, fixing whatever each reports
