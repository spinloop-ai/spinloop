## 1. Fleet file: `file` field on any node

- [ ] 1.1 Add `File string \`yaml:"file"\`` to `NodeConfig` in
      `internal/fleet/config.go`, available on either `kind`, resolved
      relative to `Config.Dir` when read (a helper alongside the existing
      path handling, not at parse time — no fleet command other than
      `deploy`/`start` needs it, and `start` must not require it).
- [ ] 1.2 Confirm `validate()` does not require the field for any kind.
- [ ] 1.3 Unit tests in `internal/fleet/config_test.go`: a `file` path
      resolves relative to the fleet file's directory on either node kind; a
      node without it parses fine (only `deploy`/`start` should care).

## 2. Extract the reusable deploy body from `remote deploy`

- [ ] 2.1 In `cmd/spinloop/remote.go`, split `runRemoteDeploy` into
      `deriveDeployTarget(spinloopArg string) (spinloop.Selection, string,
      remote.DeployConfig, env string, error)` (the existing `readSpinloop`
      alias-or-path resolution + Spinloop env application + `deployConfigFor`
      + `REMOTE` name resolution + `--allowed-cidr`/`--spinloop-version`
      validation) and `runDeploy(env string, dc remote.DeployConfig, opts
      deployOpts) (deployOutcome, error)` (plan print through registration).
- [ ] 2.2 Define `deployOpts` (dryRun, overwrite, reseed, allowedCidr,
      region) and `deployOutcome` (what to print, or a guard/failure reason)
      so a caller can render one node's result without interleaving raw
      stdout writes from concurrent goroutines.
- [ ] 2.3 Rewire `runRemoteDeploy` to call the two new functions and print
      `deployOutcome` exactly as it prints today — no behavior change for
      `spinloop remote deploy`.
- [ ] 2.4 Run the existing `cmd/spinloop/remote_deploy_test.go` suite
      unchanged and confirm it still passes against the refactor.

## 3. Resolving a node's Spinloop source

- [ ] 3.1 Add `resolveNodeSpinloop(node fleet.NodeConfig, fleetDir string)
      (arg string, source string, err error)` that tries, in order: (a)
      `node.File` resolved relative to `fleetDir`; (b) an alias registered
      under `node.Name` (`config.Load().Alias(node.Name)` — the same lookup
      `resolveAlias` makes, checked here only to decide whether to fall
      through, not to pre-resolve the path); (c) `filepath.Join(fleetDir,
      node.Name)` when that path exists as a directory. Returns the argument
      to hand `deriveDeployTarget` and a label for what resolved it (for
      reporting), or an error naming all three when none resolve.
- [ ] 3.2 Unit tests: each tier resolves independently; the alias tier wins
      over a same-named subdirectory when both exist; the error names all
      three when none resolve.

## 4. `spinloop fleet deploy` command

- [ ] 4.1 Add `fleetDeployCmd` in `cmd/spinloop/fleet.go`: `Use: "deploy"`,
      requires at least one node arg or `--all` (mutually exclusive), flags
      `--fleet`, `--all`, `--dry-run`/`-n`, `--overwrite`, `--reseed`,
      `--allowed-cidr`, `--region`, `--spinloop-version` (same deploy flags
      and help text as `remote deploy`).
- [ ] 4.2 Implement node selection: no node args and no `--all` → fail,
      listing the fleet's `kind: remote` nodes; `--all` → every `kind:
      remote` node in file order, `kind: daemon` nodes never selected and
      never mentioned; named args → exactly those, failing before any
      deploy runs if a name is unknown or names a `kind: daemon` node;
      `--all` plus named args → fail as ambiguous.
- [ ] 4.3 For each targeted node, call `resolveNodeSpinloop` (task 3.1); a
      node for which nothing resolves yields a per-node failure rather than
      aborting the others.
- [ ] 4.4 Run `deriveDeployTarget` + `runDeploy` per targeted node
      concurrently (bounded, e.g. `errgroup` or a simple worker loop keyed
      by node name — see design.md's "`fleet deploy` requires an explicit
      target"), reporting the resolved source (path or alias name)
      alongside each node's plan.
- [ ] 4.5 Render one line per targeted node (deployed / guarded / failed),
      and a summary; exit non-zero if any targeted node failed or was
      guarded without `--overwrite`.
- [ ] 4.6 Register the command in the fleet command tree and shell
      completion (`compRegister(c, "fleet", compFiles)`, node-name
      completion for positional args as `start`/`stop` already do).

## 5. `spinloop fleet start`/`stop` take multiple nodes or `--all`

- [ ] 5.1 Add `func (c *Config) OnlyNames(names []string) (*Config, error)`
      to `internal/fleet/config.go`, narrowing to several named nodes in the
      order given (unknown name fails immediately, naming the known nodes).
      Reimplement `Only(name string)` as `OnlyNames([]string{name})`; confirm
      its existing callers (`cmd/spinloop/fleet_logs.go`,
      `internal/fleet/select.go`'s `--node` pin) and tests
      (`internal/fleet/logs_test.go`) are unaffected.
- [ ] 5.2 Add a shared `runFleetDrive(cfg *fleet.Config, all bool, names
      []string, call fleet.Call) ([]fleet.NodeResult, error)` in
      `cmd/spinloop/fleet.go`, replacing `driveOneNode` (deleted): no names
      and no `--all` fails, listing the fleet's nodes; `--all` plus names
      fails as ambiguous; `--all` runs `cfg.FanOut(ctx, call)`; named nodes
      run `cfg.OnlyNames(names)` then `.FanOut(ctx, call)` (an unknown name
      fails before anything is touched).
- [ ] 5.3 Rewrite `fleetStartCmd` and `fleetStopCmd` in `cmd/spinloop/fleet.go`
      on `runFleetDrive`: `Args: cobra.ArbitraryArgs`, add `--all` to both.
      `fleetStartCmd`'s `call` closes over `cfg`, looks up `entry, _ :=
      cfg.Node(n.Name())` to recover the targeted node's `NodeConfig`; for a
      `kind: daemon` entry, calls `resolveNodeSpinloop` (on failure, returns
      a failed `NodeResult` naming all three ways a source could have been
      given — no fallback to a plain start; on success, `readSpinloop` +
      `applySpinloopEnv` + `deployConfigForNode` to derive a `dc`, then
      `n.StartWith(ctx, &dc, engineKey)`, reporting the resolved source and
      derived config); for `kind: remote`, always plain `n.Start(ctx)`.
      `fleetStopCmd`'s `call` is unchanged from today — `n.Stop(ctx)` — it
      needs no node lookup, only the new target-selection wrapper.
- [ ] 5.4 Render one line per targeted node (started/stopped, guarded,
      failed) through a shared renderer both commands call; exit non-zero
      if any targeted node failed.
- [ ] 5.5 Confirm `remoteNode.StartWith`'s existing refusal
      (`internal/fleet/remote_node.go:77-83`) means a `kind: remote` node
      is never sent a resolved config by `fleet start` — resolution is only
      ever attempted for `kind: daemon` entries.

## 6. Tests

- [ ] 6.1 `cmd/spinloop/fleet_test.go` (or a new `fleet_deploy_test.go`):
      `--all` deploys every remote node and never mentions daemon nodes;
      named args narrow the set; no target (no args, no `--all`) fails
      listing remote nodes; `--all` plus named args fails as ambiguous; an
      unknown name fails before deploying; naming a daemon node explicitly
      fails.
- [ ] 6.2 A node with no `file` field, no matching alias, and no matching
      subdirectory fails only that node in `fleet deploy`; the rest still
      deploy.
- [ ] 6.3 A node resolved via alias and a node resolved via subdirectory
      both deploy correctly in the same run; a node with both an alias and a
      same-named subdirectory uses the alias.
- [ ] 6.4 One node already registered/live is guarded without `--overwrite`
      while a sibling node still deploys; the command exits non-zero.
- [ ] 6.5 `--dry-run` prints every targeted node's plan and performs no AWS
      calls (assert via the existing seams: `deployDiscoverFn`,
      `remoteDeployFn`, etc. left uncalled).
- [ ] 6.6 A node deployed via `fleet deploy` and the same Spinloop file
      deployed via standalone `remote deploy` produce identical
      `remote.DeployConfig` and registration output (parity test using
      `deriveDeployTarget` directly).
- [ ] 6.7 `fleet start` on a `kind: daemon` node with a resolved `file`
      field, a resolved alias, and a resolved subdirectory each derive and
      push the expected `StartWith` config; report includes the resolved
      source.
- [ ] 6.8 `fleet start` on a `kind: daemon` node with no resolvable source
      fails, naming all three ways a source could have been given — assert
      `Start` and `StartWith` are both never invoked.
- [ ] 6.9 `fleet start` on a `kind: remote` node with a resolvable source
      still calls plain `Start`, never `StartWith`.
- [ ] 6.10 `internal/fleet/config_test.go`: `OnlyNames` narrows to several
      named nodes in the order given; an unknown name among several fails
      immediately, naming the known nodes; `Only`'s existing behavior and
      tests (`internal/fleet/logs_test.go`) are unaffected.
- [ ] 6.11 `cmd/spinloop/fleet_test.go`: `fleet start gpu-a gpu-b` starts
      both (independently — one succeeding while the other fails does not
      abort the first); `fleet start --all` starts every node in the file,
      daemon and remote alike; `fleet start` with no args and no `--all`
      fails listing the nodes; `--all` plus node args fails as ambiguous; an
      unknown name among several fails before starting any.
- [ ] 6.12 `cmd/spinloop/fleet_test.go`: the same set, mirrored for `fleet
      stop` (`stop gpu-a gpu-b`, `stop --all`, no-target failure, `--all`
      plus names ambiguous, unknown name among several) — `stop`'s `call`
      needs no Spinloop-resolution coverage since it takes no config.
- [ ] 6.13 `go test ./... -cover` stays at or above the project's 80% floor.

## 7. Docs and examples

- [ ] 7.1 `docs/commands/fleet.md`: document the `file` field and the
      alias/subdirectory fallbacks (generalized beyond "remote environments"
      to any node), add a `## Deploying remote nodes` section (command,
      flags, `--all`/named-arg requirement, guard/failure reporting), and
      rewrite the "Starting and stopping" section: both `start` and `stop`
      now take one or more node names or `--all` (no more "one node at a
      time"), and `start` on a `kind: daemon` node now needs a resolvable
      Spinloop source or fails for it — **BREAKING**, called out as such.
- [ ] 7.2 `docs/commands/remote.md`: cross-reference `fleet deploy` as the
      batch alternative to running `remote deploy` once per environment.
- [ ] 7.3 Every existing example with a `kind: daemon` node
      (`examples/fleet-local/`, `examples/fleet-docker/`,
      `examples/fleet-mixed/`) needs a `file` field, a matching alias, or a
      matching subdirectory added for each such node, or `spinloop fleet
      start` breaks for it — this is required, not optional, for the
      examples to keep working. Also extend `examples/fleet-remote/` (or
      `fleet-mixed/`) with a node using each resolution tier — one with an
      explicit `file` field, one relying on a same-named subdirectory — so
      it is deployable via `fleet deploy`. Update each example's README
      accordingly.

## 8. Validation

- [ ] 8.1 `gofmt -l .` clean.
- [ ] 8.2 `go build ./...` and `go vet ./...` clean.
- [ ] 8.3 Manually exercise `spinloop fleet deploy --dry-run --all` against
      `examples/fleet-remote/` (or `fleet-mixed/`) and confirm the printed
      plan matches what standalone `remote deploy --dry-run` prints for the
      same Spinloop file.
- [ ] 8.4 Manually exercise `spinloop fleet start <daemon-node>` against each
      updated example (`fleet-local`, `fleet-docker`, `fleet-mixed`) and
      confirm the engine starts with the resolved config; confirm a node
      with no resolvable source fails naming the three ways one could have
      been given, rather than starting.
- [ ] 8.5 Manually exercise `spinloop fleet start --all` and `spinloop fleet
      stop --all` against `fleet-docker` or `fleet-mixed` (several nodes,
      mixed kinds) and confirm every node starts/stops in one command each.
