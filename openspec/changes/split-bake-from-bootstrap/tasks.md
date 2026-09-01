## 1. Share the bootstrap machinery

- [ ] 1.1 Rename the generic bootstrap-prefixed seams and helpers in `cmd/spinloop/remote_bootstrap.go` to shared names both commands use (the step runner and its `*Fn` test seams, `waitForBake`), keeping them as package variables so tests stay hermetic
- [ ] 1.2 Extract the source-resolution sequence (resolve ref, default/explicit dir, download, prune-other-refs-on-success) into a shared helper that bootstrap and bake both call

## 2. Strip the bake from bootstrap

- [ ] 2.1 Remove the `--runners`, `--wait`, and `--force-bake` flags and the bake loop from `runBootstrapSequence` (sequence becomes install → `cdk bootstrap` → `deploy:image` → `deploy`), and drop the now-dead `cdk.json` `context.runners` write
- [ ] 2.2 Update the plan rendering (no bake lines; the Image Builder bullet no longer claims baked AMIs) and the success output to signpost `spinloop remote bake` as the next step ahead of `spinloop remote deploy`; update the command's `Short`/`Long`

## 3. Add `spinloop remote bake`

- [ ] 3.1 New `cmd/spinloop/remote_bake.go`: `bake [runner...]` with positional runners (default both, validated against the accepted runner set), a `--no-wait` flag (the wait is the default), and `--ref`/`--dir`/`--package-manager` flags matching bootstrap's; register it on the remote command group and give the positional argument runner-name completion
- [ ] 3.2 Bake body: preflight (Node 22+, a package manager, resolvable AWS credentials), fail early naming `spinloop remote bootstrap` when the control-plane stack is not deployed, shared source resolution, run install-if-needed plus one bake step per runner, `waitForBake` polling by default (skipped with `--no-wait`, which reports how to check on the builds), and prune other refs on success when the default location was used

## 4. internal/remote

- [ ] 4.1 Update the `BakedRunners` comment: it now serves `spinloop remote bake --wait` rather than `bootstrap --wait`

## 5. Tests

- [ ] 5.1 Update `cmd/spinloop/remote_bootstrap_test.go`: expected sequence without bake steps, plan output without bakes, the success signpost, drop the `--runners`/`--force-bake`/`--wait` tests and the `runners` context-write assertion
- [ ] 5.2 New `cmd/spinloop/remote_bake_test.go` driving the shared seams hermetically: default both runners, a single runner, an unknown runner rejected before anything runs, no control plane failing early naming bootstrap, the default wait polling to available (and the timeout path), `--no-wait` returning once the bakes are queued, and default-location prune vs an explicit `--dir`

## 6. Docs

- [ ] 6.1 `docs/commands/remote.md`: rewrite the bootstrap section (drop the `--runners`/`--wait` examples) and add a `bake` section (usage, runner default, `--wait`, the fail-early when the control plane is missing)
- [ ] 6.2 `remote/README.md`: the Deploy section's journey becomes bootstrap → bake → deploy; fix the inline comment that says bootstrap does "control-plane stack + pipelines + bakes"
- [ ] 6.3 `docs/env-vars.md`: note that `SPINLOOP_REMOTE_PACKAGE_MANAGER` applies to `bootstrap` and `bake` alike

## 7. Verification

- [ ] 7.1 `gofmt -w ./...`, `go vet ./...`, and `go test ./... -cover` green with total coverage ≥ 80%
