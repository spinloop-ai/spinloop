## 1. Fleet file: `spinloop` field on remote nodes

- [ ] 1.1 Add `Spinloop string \`yaml:"spinloop"\`` to `NodeConfig` in
      `internal/fleet/config.go`, resolved relative to `Config.Dir` when
      read (a helper alongside the existing path handling, not at parse
      time — other kinds ignore it and validation must not require it).
- [ ] 1.2 Confirm `validate()` does not require the field for any kind, and
      that a `kind: daemon` node declaring it parses without effect.
- [ ] 1.3 Unit tests in `internal/fleet/config_test.go`: a remote node with a
      `spinloop` path resolves relative to the fleet file's directory; a
      daemon node declaring the field is unaffected; a remote node without
      it parses fine (only `fleet deploy` should care).

## 2. Extract the reusable deploy body from `remote deploy`

- [ ] 2.1 In `cmd/spinloop/remote.go`, split `runRemoteDeploy` into
      `resolveDeployTarget(spinloopPath string) (spinloop.Selection,
      remote.DeployConfig, env string, error)` (Spinloop env application +
      `deployConfigFor` + `REMOTE` name resolution + `--allowed-cidr`/
      `--spinloop-version` validation) and `runDeploy(env string, dc
      remote.DeployConfig, opts deployOpts) (deployOutcome, error)` (plan
      print through registration).
- [ ] 2.2 Define `deployOpts` (dryRun, overwrite, reseed, allowedCidr,
      region) and `deployOutcome` (what to print, or a guard/failure reason)
      so a caller can render one node's result without interleaving raw
      stdout writes from concurrent goroutines.
- [ ] 2.3 Rewire `runRemoteDeploy` to call the two new functions and print
      `deployOutcome` exactly as it prints today — no behavior change for
      `spinloop remote deploy`.
- [ ] 2.4 Run the existing `cmd/spinloop/remote_deploy_test.go` suite
      unchanged and confirm it still passes against the refactor.

## 3. `spinloop fleet deploy` command

- [ ] 3.1 Add `fleetDeployCmd` in `cmd/spinloop/fleet.go`: `Use: "deploy"`,
      `Args: cobra.ArbitraryArgs`, flags `--fleet`, `--dry-run`/`-n`,
      `--overwrite`, `--reseed`, `--allowed-cidr`, `--region`,
      `--spinloop-version` (same flags and help text as `remote deploy`).
- [ ] 3.2 Implement node selection: no args → every `kind: remote` node in
      file order; named args → exactly those, failing before any deploy runs
      if a name is unknown or names a `kind: daemon` node explicitly; a
      `kind: daemon` node swept in only by the no-args case is reported
      skipped and excluded from the deploy set.
- [ ] 3.3 For each targeted node, resolve its Spinloop path via `NodeConfig`
      relative to the fleet file's directory; a node with no `spinloop`
      field yields a per-node failure naming the missing field rather than
      aborting the others.
- [ ] 3.4 Run `resolveDeployTarget` + `runDeploy` per targeted node
      concurrently (bounded, e.g. `errgroup` or a simple worker loop keyed
      by node name — see design.md's "Node selection and concurrency").
- [ ] 3.5 Render one line per targeted node (deployed / skipped / guarded /
      failed), and a summary; exit non-zero if any targeted node failed or
      was guarded without `--overwrite`.
- [ ] 3.6 Register the command in the fleet command tree and shell
      completion (`compRegister(c, "fleet", compFiles)`, node-name
      completion for positional args as `start`/`stop` already do).

## 4. Tests

- [ ] 4.1 `cmd/spinloop/fleet_test.go` (or a new `fleet_deploy_test.go`):
      no-args deploys every remote node and skips daemon nodes; named args
      narrow the set; an unknown name fails before deploying; naming a
      daemon node explicitly fails.
- [ ] 4.2 A missing `spinloop` field on one targeted node fails only that
      node; the rest still deploy.
- [ ] 4.3 One node already registered/live is guarded without `--overwrite`
      while a sibling node still deploys; the command exits non-zero.
- [ ] 4.4 `--dry-run` prints every targeted node's plan and performs no AWS
      calls (assert via the existing seams: `deployDiscoverFn`,
      `remoteDeployFn`, etc. left uncalled).
- [ ] 4.5 A node deployed via `fleet deploy` and the same Spinloop file
      deployed via standalone `remote deploy` produce identical
      `remote.DeployConfig` and registration output (parity test using
      `resolveDeployTarget` directly).
- [ ] 4.6 `go test ./... -cover` stays at or above the project's 80% floor.

## 5. Docs and examples

- [ ] 5.1 `docs/commands/fleet.md`: document the `spinloop` field under
      "Remote environments" and add a `## Deploying remote nodes` section
      with the command, its flags, and the skip/guard/failure reporting.
- [ ] 5.2 `docs/commands/remote.md`: cross-reference `fleet deploy` as the
      batch alternative to running `remote deploy` once per environment.
- [ ] 5.3 Extend `examples/fleet-remote/` (or `examples/fleet-mixed/`) with a
      `spinloop` field on its remote node(s) so the example is deployable
      via `fleet deploy`, and update its README accordingly.

## 6. Validation

- [ ] 6.1 `gofmt -l .` clean.
- [ ] 6.2 `go build ./...` and `go vet ./...` clean.
- [ ] 6.3 Manually exercise `spinloop fleet deploy --dry-run` against
      `examples/fleet-remote/` (or `fleet-mixed/`) and confirm the printed
      plan matches what standalone `remote deploy --dry-run` prints for the
      same Spinloop file.
