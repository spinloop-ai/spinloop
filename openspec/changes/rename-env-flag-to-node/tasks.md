# Tasks

## 1. Rename the flag registrations

- [x] 1.1 In `cmd/spinloop`, register `--node`/`-n` in place of `--env`/`-e` on
  `apply`, `unapply` (`main.go`), the launch builder (`code.go`), `status.go`,
  `logs.go`, `dashboard.go`, the fleet group (`fleet.go`), and the cloud group
  (`cloud.go`, `cloud_schedule.go`), renaming the bound variables and the
  `envFlagUsage`/`envFlagTargetUsage` help constants to node wording. Verify:
  `go build ./...` passes and no `StringVar*` registers a flag named `"env"`
  (`rg '"env"' cmd/spinloop --glob '!*_test.go'` is empty).
- [x] 1.2 Merge `routeOptions.envName` into `routeOptions.node` in
  `cmd/spinloop/route.go` and adapt `code.go`'s launch branches per
  design.md (environment only when no fleet is in force; `--node` with a
  discovered fleet routes). Verify: `go build ./...` passes.
- [x] 1.3 Drop `checkOneTarget` from `applyRoutedSpinloop` and key the cloud
  fetch (`fetchCloudEnv`) and `applySelection`'s environment slot on
  "`--node` set and routing produced no choice". Verify: `go build ./...`
  passes.
- [x] 1.4 Re-word the remaining user-facing strings that name the flag or its
  one-target conflict: `target.go`'s conflict sentence, registry errors in
  `main.go`/`cloud.go` ("run `spinloop cloud deploy --node …`"), and every
  `--env` occurrence in help text. Verify: `rg -- '--env|\-e\b' cmd/spinloop
  --glob '!*_test.go'` shows only `--print-env`, `--token-env`, and `.env`.

## 2. Completion

- [x] 2.1 Register `compEnvs` under `node` (and short `n`) everywhere it was
  registered under `env`; add `-n` to `launchValueFlags` and make
  `launchSlot` offer registered environment names (plus the in-force fleet's
  node names where readable) for `--node`/`-n`, never erroring. Verify:
  `go test ./cmd/spinloop/ -run Comp` passes and `spinloop __complete code
  --node ''` lists environments.

## 3. Tests

- [x] 3.1 Rename `--env`/`-e` args and message assertions across
  `cmd/spinloop/*_test.go`; add cases for: `--env` now rejected as unknown,
  bare `--node` with no fleet configuring from the environment, `--node` +
  discovered `fleet.yaml` routing (gateway served, and failing for the
  Spinloop when the fleet has none), `--node` naming a non-fleet name with a
  fleet in force failing without registry fallback. Verify: `go test ./...`
  passes.
- [x] 3.2 Run `go vet ./...`, `gofmt -l .` (empty), and
  `go test ./... -cover` with total ≥ 80%. Verify: all three clean.

## 4. Docs and examples

- [x] 4.1 Update `docs/` pages and `examples/` READMEs/Spinloops that show
  `--env` or `-e` for targeting to `--node`/`-n`, leaving `--print-env`,
  `--token-env`, `ENV`, and `.env` alone. Verify: `rg -- '--env|-e\b' docs
  examples` shows only environment-variable spellings.

## 5. Integration

- [x] 5.1 Exercise the renamed surface end-to-end: build the binary, run a
  fleet-docker-example-style flow by hand or via the example's script
  (`examples/fleet-docker/`), confirming launch, status and pin all work with
  `--node`/`-n` and that `--env` errors everywhere. Verify: the flow
  completes and prints the announced node. (Done against stub daemons —
  Docker unavailable locally; CI's fleet-integration job covers the Docker
  flow.)

## 6. Review revision (draft PR)

- [x] 6.1 Let `--fleet` and `--node` pair instead of conflict: `--fleet`
  names the fleet, `--node` narrows to the node within it, named or
  discovered. A name a *named* file lacks fails naming the file and its
  nodes — registry fallback stays only beside a discovered `fleet.yaml`.
  Delete `checkOneTarget`; feed `--node` completion from the fleet in force
  (`compNodes`). Verify: `go test ./cmd/spinloop`, spec strict-valid, and
  the stub-daemon driver's `--fleet … -n …` pair and ghost-name checks.
