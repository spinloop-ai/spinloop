## 1. The capabilities

- [x] 1.1 Add `Coster` to `internal/fleet` beside `ProgressStarter` and
      `Keeper`: what a node has cost so far and its hourly rate, answered by
      `remoteNode` from its own instance type and region (design D1). Verify a
      unit test over both kinds — the cloud node answers, the daemon does not
      implement it.
- [x] 1.2 Add `SourceLogger`: reading a node's log by source, time window and
      instance. Verify the cloud node answers all three and the daemon does
      not implement it.
- [x] 1.3 Add `InstanceReporter`: the instance a node runs on — its id, type,
      the spinloop release on it, its endpoint address and retention deadline.
      Verify it is answered from the replies the node already holds, with no
      extra call (design D3), and that `statsFromRemote` stops dropping the
      version and instance type.
- [x] 1.4 Verify no caller branches on node kind: a test or a grep confirming
      the capabilities are reached only by type assertion.

## 2. The verbs

- [x] 2.1 Add `cmd/spinloop/metrics.go`: a root-registered `metrics` taking
      `--env`/`--fleet`, `--format`, `--watch` and `--cost`, resolving through
      `resolveFleetTarget` and rendering through the existing formatters.
      Verify each format renders for a fleet and for a single environment.
- [x] 2.2 Add `cmd/spinloop/logs.go` the same way, with `--follow`, `--limit`,
      `--format`, and the `--source`/`--since`/`--instance` the capability
      answers. Verify `-f` remains `--follow` and the fleet file is long-form
      only.
- [x] 2.3 Wire `--cost` to `Coster`: priced nodes carry the figure, the rest
      render as they would without the flag, and a target with none succeeds
      (design D2). Verify with a mixed target and an all-daemon one.
- [x] 2.4 Wire `--source`/`--since`/`--instance` to `SourceLogger` on the same
      terms. Verify a mixed target reads the environment's boot log and the
      daemon's ordinary output.
- [x] 2.5 Read a Spinloop given to either verb for its `ENV` instructions and
      adjacent `.env` only, never to select a target (design D4). Verify a
      Spinloop whose `ENV` supplies `SPINLOOP_REMOTE_*` configures the command.
- [x] 2.6 Register both at the root with the completion the fleet spellings
      had. Verify the completion test covers them.

## 3. Removing the old spellings

- [x] 3.1 Delete `fleetMetricsCmd` and `fleetLogsCmd` and unregister them,
      keeping their renderers. Verify `spinloop fleet --help` lists neither.
- [x] 3.2 Delete `remoteStatusCmd`, `remoteMetricsCmd`, `remoteLogsCmd` and
      their bodies, keeping the formatters the top-level verbs use. Verify
      `spinloop remote --help` lists none of the three.
- [x] 3.3 Signpost all five moved spellings through `movedSubcommands`, giving
      the `remote` group the same `Args: groupArgs` the fleet group has
      (design D5). Verify each names its replacement and an unknown subcommand
      in either group still gets cobra's own error.

## 4. Consumers, docs and verification

- [x] 4.1 Update every invocation of the five moved spellings across `docs/`,
      `README.md` and `examples/` — including the two CI-run `run-tests.sh`
      scripts, whose `fleet()` shell helper means a bare rename breaks them.
      Verify `bash -n` on each script and that none invokes a removed
      spelling.
- [x] 4.2 Add `docs/commands/metrics.md` and `logs.md`, each saying which
      flags apply to which node kinds (required by the capability
      requirement), and point `fleet.md` and `remote.md` at them. Verify
      `docs/README.md`'s command table lists both.
- [x] 4.3 Verify nothing an operator could get from the removed commands is
      unreachable: the cost, the version, the log sources, the `ENV` path, and
      the endpoint address via `remote env`.
- [x] 4.4 Run `gofmt -l .` (expect no output), `go vet ./...` and
      `go test ./... -cover`, confirming total coverage is unchanged and still
      >= 80%.
