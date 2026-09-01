# AGENTS.md

This file provides guidance to coding agents, such as Claude Code, when working with code in this repository.

## What this is

`spinloop` covers a model's whole lifecycle — deploy it, host it, watch it, and point a coding agent at it — driven from one declarative `Spinloop` file:

- **Harness configuration** — deep-merges provider settings into a coding agent's config. Three harnesses are supported: [opencode](https://opencode.ai) (config under `${XDG_CONFIG_HOME:-$HOME/.config}/opencode`), [Pi](https://github.com/earendil-works/pi) (`~/.pi/agent/models.json`), and [lucinate](https://github.com/lucinate-ai/lucinate) (`~/.lucinate/connections.json`). The harness is chosen at runtime — never baked into a Spinloop file — so the same selection applies to any of them.
- **Local hosting** — `spinloop serve` runs an inference engine (llama.cpp, oMLX) directly; `spinloop daemon` supervises one instead, exposing an HTTP control API for starting/stopping it and reading its status, metrics, and logs.
- **Cloud deployment** — `spinloop remote` drives a scale-to-zero GPU instance through its own AWS control plane (`remote/`): starts it on demand, deploys a model to it, and stops it when idle.
- **Fleet monitoring and control** — `spinloop fleet` observes and drives every engine you run — local daemons and remote environments alike — across machines from one place, including an interactive dashboard, and can route a harness launch to whichever node already has (or can load) the wanted model.

## Where the design lives

Architecture and design decisions are specified in **`openspec/specs/`** — one directory per feature area (`fleet-routing`, `alias-registry`, `provider-catalog`, `opencode-integration`, …), each a `spec.md` of `SHALL` requirements with scenarios. That is the source of truth for *behavior*: read the relevant spec before changing how something works, and update it as part of the change (see `openspec/changes/` and the `openspec-*` skills).

Implementation gotchas and cross-cutting rationale that aren't behavior requirements — things worth knowing before you touch the code, not things a user or a spec scenario cares about — live in [`docs/internals.md`](docs/internals.md).

User-facing documentation (install, commands, the Spinloop file format, the HTTP API) lives under `docs/` — see [`docs/README.md`](docs/README.md).

This file stays an orientation map: what's where, and which spec or doc governs it.

## Commands

```sh
go test ./...                  # run the suite
go test ./... -cover           # with coverage (keep total >= 80%)
go vet ./...                   # vet
go build -o spinloop ./cmd/spinloop   # build the CLI binary
gofmt -w ./...                 # format
```

Run a single test: `go test -run TestWriteConfig_Idempotent ./...`

## Layout

The binary lives under `cmd/`; domain logic is split into `internal/` packages so each concern is isolated and independently testable. Spec pointers below name the `openspec/specs/` directory that governs a package's behavior — read it before changing that behavior.

- `cmd/spinloop/commands.go` + `main.go` — the Cobra command tree and command bodies: `add`/`remove`/`apply`/`unapply`/`show`/`export`/`harness`/`alias`/`unalias`. (`harness-management`, `alias-registry`)
- `cmd/spinloop/serve.go` — the `serve` command: engine dispatch (llama.cpp, oMLX) and the Spinloop/preset→flags mappings. (`local-serving`, `inference-runners`)
- `cmd/spinloop/complete.go` — tab completion, built on Cobra's `__complete` engine. (`shell-completion`)
- `internal/config` — spinloop's own config file (`${XDG_CONFIG_HOME:-~/.config}/spinloop/config.json`): default-harness preference and the alias registry. A leaf package — stdlib only, never imports `internal/spinloop`. (`config-location`, `alias-registry`)
- `internal/spinloop` — the Spinloop file format and the shared `Selection` type. A pure grammar leaf: no I/O. (`spinloop-files`)
- `internal/spinloopsrc` — resolves and fetches a Spinloop-family reference (path, `PRESET`, path-form `REMOTE`) that may be local or an `http(s)` URL. (`remote-spinloop-sources`)
- `internal/harness` — the harness abstraction: the `Harness` interface, the opencode/Pi/lucinate adapter registry, and runtime resolution via `harness.Resolve`. Start here when adding another harness. (`harness-management`)
- `internal/catalog` — the embedded provider catalogue (`//go:embed providers.yaml`) and the block builders that turn a selection into an opencode or Pi provider entry. (`provider-catalog`, `provider-selection`)
- `internal/opencode` — opencode config IO: JSONC read/merge/write via RFC 6902 patches on the hujson AST, preserving comments and sibling config. (`opencode-integration`)
- `internal/pi` — Pi's `models.json` IO: deep-merge of one managed provider, preserving siblings and unknown fields. (`pi-integration`)
- `internal/lucinate` — lucinate's `connections.json` IO: one managed connection, no secret ever written to disk. (`lucinate-integration`)
- `cmd/spinloop/fleet.go`, `metrics_render.go`, `status_render.go`, `fleet_dashboard.go`, `dashboard_*.go` — the `fleet` command group and its Bubble Tea dashboard (the CLI's only TUI). (`fleet-client`, `fleet-config`)
- `internal/fleet` — the fleet client: the `Node` interface (`daemonNode`/`remoteNode`), concurrent fan-out, and routing/waking a node for a launch. (`fleet-client`, `fleet-config`, `fleet-routing`, `remote-node`)
- `examples/fleet-docker/` — a runnable multi-node fleet that doubles as the fleet integration test, run per PR by CI. (`fleet-docker-example`)
- `cmd/spinloop/remote.go` + `internal/remote` — the `remote` command group and the scale-to-zero cloud GPU control plane (SigV4-signed Lambda Function URL calls — the repo's only AWS/network dependency). (`remote-environments`, `endpoint-lifecycle`, `endpoint-provisioning`, `remote-endpoint`, `remote-seed`, `weight-seeding`, `remote-keep`, `remote-start-probe`)
- `internal/daemon` — the engine supervisor and the HTTP control API. `Routes()` in `api.go` is checked against `docs/openapi.yaml` by `openapi_test.go` — keep them in sync when adding an endpoint. (`daemon-api`, `daemon-api-contract`, `engine-activity`, `engine-metrics`, `api-logging`, `serve-daemon`)
- `internal/contextsize` — parses human-friendly sizes (`128k`, `1.5m`) for `CONTEXT`/`OUTPUT`.
- `internal/preset` — parses llama.cpp-style preset `.ini` files, dialect-aware (LlamaCpp vs. OMLX). (`inference-runners`)
- `internal/catalog/providers.yaml` — externalised provider plumbing (URLs, key env vars, npm packages) — no model ids. Add providers here, not in Go. (`provider-catalog`)
- `examples/` — runnable guides, each a directory with a README and a `Spinloop`.
- `remote/` — the TypeScript CDK project `spinloop remote` drives (Lambdas, EC2 Image Builder, S3 weights), built and tested by pnpm with its own CI job. **Public repo:** nothing identifying a deployment (account ids, ARNs, hosts, bucket names) may be committed — enforced by `scripts/check-no-cloud-identifiers.sh`.

## Invariants to keep

opencode merges stay idempotent and never drop unrelated config or comments; Pi and lucinate merges preserve sibling entries and unknown fields; no harness ever writes a resolved secret to disk; config files that may hold a key are written `0600`; the harness never appears in a Spinloop, and neither does an alias — both are machine-local; spinloop's own `config.json` is only ever written read-modify-write; `__complete` never returns an error and never writes to stderr, whatever the state of the config or catalogue.

## Workflow

Warn the user if they are going to merge a branch with un-archived openspec changes.
