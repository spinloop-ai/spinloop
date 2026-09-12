## 1. Go: instance type in the deploy config and a shared validator

- [ ] 1.1 Add `InstanceType string` (JSON key `instanceType`, `omitempty`) to
  `DeployConfig` in `internal/remote/remote.go`. Verify with a unit test that a
  config carrying the type marshals an `instanceType` key and a config without
  it omits the key (matching the `SpinloopVersion` pattern).
- [ ] 1.2 Add an exported EC2 instance-type pattern and validator in
  `internal/remote` (shape `family.size`, lowercase, family may be hyphenated).
  Verify with a unit test that it accepts `g6e.xlarge`, `u7i-6tb.112xlarge`,
  `mac2-m2.2xlarge`, `trn1.2xlarge` and rejects empty, `g6exlarge`,
  `G6E.xlarge`, and `g6e.xlarge.extra`.

## 2. Go: the fleet file field

- [ ] 2.1 Add `InstanceType string` (YAML key `instance-type`) to `NodeConfig`
  in `internal/fleet/config.go`. Verify the field parses from a `fleet.yaml`
  listing a `kind: remote` node with `instance-type: g6e.2xlarge`.
- [ ] 2.2 In `validate()`, reject `instance-type` on a `kind: daemon` node and
  check its shape (via the `internal/remote` validator) on a `kind: remote`
  node. Verify with tests: a daemon node naming one is rejected naming the
  node; a remote node with a malformed value is rejected naming the node and
  value; a remote node with no value or a valid value parses.

## 3. Go: `remote deploy --instance-type`

- [ ] 3.1 Add an `instanceType string` field to `deployOpts` and an
  `--instance-type` flag to `remoteDeployCmd`, threading it through
  `runRemoteDeploy` into the `deployOpts` passed to `runDeploy`. Verify
  `spinloop remote deploy --help` lists the flag.
- [ ] 3.2 In `runDeploy`, when `opts.instanceType` is non-empty, validate it
  (naming the value on failure, before any send) and set `dc.InstanceType`;
  treat an empty/whitespace value as absent. Verify with a test that a
  malformed value fails before the deploy seam is reached and a valid value is
  set on the derived config.
- [ ] 3.3 Print the resolved instance type in the deploy plan (the type when
  set, otherwise a statement that the environment launches as the control
  plane's default), alongside the runner and model. Verify `remote deploy
  --dry-run --instance-type g6e.2xlarge` prints `g6e.2xlarge` and a run without
  the flag prints the default statement.

## 4. Go: `fleet deploy` threads the node's value

- [ ] 4.1 In `deployOneNode`, set the targeted node's `InstanceType` into the
  `deployOpts` passed to `runDeploy` (per-node; a node naming none leaves it
  empty). Verify with a test that `fleet deploy --dry-run` for a node declaring
  `instance-type` shows that type in its plan and a node declaring none shows
  the default.

## 5. TypeScript: parse and validate the field

- [ ] 5.1 Add `instanceType?: string` to the `DeployConfig` interface and parse
  + validate it in `parseDeployConfig` in `remote/lambda/shared/deploy-config.ts`
  (absent → `undefined`; present → non-empty string matching the same
  `family.size` shape, else a thrown error naming the value). Verify by
  extending `remote/test/deploy-config.test.ts`: a config with the type parses
  it, one without leaves it `undefined`, and a malformed value throws.
- [ ] 5.2 Confirm `writeDeployConfig`/`readDeployConfig` round-trip the field
  (they serialise the whole config). Verify with a test that a config written
  with `instanceType` reads it back.

## 6. TypeScript: the start launches with the stored type

- [ ] 6.1 In `remote/lambda/start/index.ts`, launch with
  `deployConfig.instanceType` when present, else the `INSTANCE_TYPE` env var,
  in the fresh-launch path. Verify by extending `remote/test/start-launch.test.ts`:
  a deploy config naming a type launches as it, and one naming none launches as
  the env-var default.
- [ ] 6.2 Update the no-capacity reply to name the instance type it was trying
  (rather than a hard-coded `g6e`). Verify the no-capacity test asserts the
  type appears in the message.

## 7. Whole-repo verification

- [ ] 7.1 Run `gofmt -l` over the touched Go files and confirm no output; run
  `go vet ./...` and confirm it is clean.
- [ ] 7.2 Run `go test ./... -cover` and confirm the suite passes with total
  coverage still >= 80%.
- [ ] 7.3 From `remote/`, run `pnpm build` (tsc type-check) and `pnpm test`
  (vitest) and confirm both pass.

## 8. Docs and examples

- [ ] 8.1 Document `instance-type` in the `fleet.yaml` reference (remote-only,
  the shape it must take, that it is deployed by `fleet deploy`) and
  `--instance-type` in the `remote deploy` reference (recorded per environment,
  applied on the next fresh launch, default when omitted). Verify the docs
  build/lint if the repo has a docs check, else re-read for accuracy.
- [ ] 8.2 Add an `instance-type` to a `kind: remote` node in the fleet/remote
  example (`examples/fleet-remote/fleet.yaml`) with a short comment, so the
  field is discoverable. Verify the example still matches the documented format.
