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

## 5. `spinloop fleet start` uses a daemon node's resolved source

- [ ] 5.1 Change `driveOneNode`'s call closure signature in
      `cmd/spinloop/fleet.go` from `func(ctx, fleet.Node) fleet.NodeResult`
      to `func(ctx, *fleet.Config, fleet.NodeConfig, fleet.Node)
      fleet.NodeResult`, passing the resolved `NodeConfig` and the fleet
      `*Config` through. Update `fleetStopCmd`'s closure to ignore the new
      parameters (unchanged behavior).
- [ ] 5.2 Update `fleetStartCmd`'s closure: for a `kind: daemon` entry, call
      `resolveNodeSpinloop`; on success, `readSpinloop` + `applySpinloopEnv`
      + `deployConfigForNode` to derive a `dc`, report the resolved source
      and derived config, then `n.StartWith(ctx, &dc, engineKey)`. On
      failure to resolve, or for a `kind: remote` entry, fall through to
      today's plain `n.Start(ctx)` unchanged.
- [ ] 5.3 Confirm `remoteNode.StartWith`'s existing refusal
      (`internal/fleet/remote_node.go:77-83`) means a `kind: remote` node
      is never sent a resolved config by `fleet start`, even if one
      resolves for it — resolution is attempted for `kind: daemon` entries
      only.

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
      falls back to a plain `Start` call — assert `StartWith` is never
      invoked.
- [ ] 6.9 `fleet start` on a `kind: remote` node with a resolvable source
      still calls plain `Start`, never `StartWith`.
- [ ] 6.10 `go test ./... -cover` stays at or above the project's 80% floor.

## 7. Docs and examples

- [ ] 7.1 `docs/commands/fleet.md`: document the `file` field and the
      alias/subdirectory fallbacks (generalized beyond "remote environments"
      to any node), add a `## Deploying remote nodes` section (command,
      flags, `--all`/named-arg requirement, skip/guard/failure reporting),
      and update the "Starting and stopping" section to describe the
      resolved-source push for daemon nodes.
- [ ] 7.2 `docs/commands/remote.md`: cross-reference `fleet deploy` as the
      batch alternative to running `remote deploy` once per environment.
- [ ] 7.3 Extend `examples/fleet-remote/` (or `examples/fleet-mixed/`) with a
      node using each resolution tier — one with an explicit `file` field,
      one relying on a same-named subdirectory — so the example is
      deployable via `fleet deploy`, and add a `kind: daemon` node with a
      resolvable source to demonstrate `fleet start`'s new behavior. Update
      the example's README accordingly.

## 8. Validation

- [ ] 8.1 `gofmt -l .` clean.
- [ ] 8.2 `go build ./...` and `go vet ./...` clean.
- [ ] 8.3 Manually exercise `spinloop fleet deploy --dry-run --all` against
      `examples/fleet-remote/` (or `fleet-mixed/`) and confirm the printed
      plan matches what standalone `remote deploy --dry-run` prints for the
      same Spinloop file.
- [ ] 8.4 Manually exercise `spinloop fleet start <daemon-node>` against a
      local daemon (e.g. `examples/fleet-local/` or `fleet-docker/`) with a
      resolvable source and confirm the engine starts with that config, then
      again with no resolvable source and confirm it starts exactly as
      before this change.
