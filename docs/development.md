# Development

Building `spinloop`, the checks a change has to pass, and where to add a
provider or a harness. For what the tool does, see the [documentation
index](README.md); for how it is required to behave, `openspec/specs/`; for
mistakes already made here, [implementation notes](internals.md).

## Build and test

`spinloop` is a Go CLI with no runtime dependencies — one static binary.

```sh
go build -o spinloop ./cmd/spinloop   # build the binary
go test ./...                         # run the suite
go test ./... -cover                  # with coverage (kept >= 80%)
go vet ./...                          # vet
gofmt -w ./...                        # format
```

A single test: `go test -run TestWriteConfig_Idempotent ./...`

## Layout

The command tree lives under `cmd/`; the domain logic is split into `internal/`
packages so each concern is isolated and independently testable.
[`AGENTS.md`](../AGENTS.md) is the map — one line per package, each naming the
spec in `openspec/specs/` that governs its behaviour.

## Adding a provider or model

A data change rather than a code one. `internal/catalog/providers.yaml` holds
every provider's base URL, key environment variable and package — no model ids —
and the file is commented with its schema. Add yours there and rebuild.

The catalogue is embedded into the binary at build time, so a `spinloop` you
built earlier keeps applying the old one however you edit the file; rebuild
before testing a change. To skip the rebuild while iterating, write the
catalogue out with [`spinloop init-providers`](commands/init-providers.md) and
read it at run time with `--providers` or `SPINLOOP_PROVIDERS`. The reasoning is
in [implementation notes](internals.md).

## Adding a harness

Start at the `Harness` interface in `internal/harness`, which holds the adapter
registry for opencode, Pi and lucinate. [`AGENTS.md`](../AGENTS.md) walks
through the contract and the invariants every adapter keeps — merges stay
idempotent, sibling config and comments survive, and no resolved secret is ever
written to disk.

## Before opening a pull request

- `go test ./...` green and `gofmt -w ./...` clean.
- A change in behaviour is specified in `openspec/specs/` as part of the change,
  not afterwards — see the `openspec/changes/` workflow.
- The `.env` file and the built binary are git-ignored; neither belongs in a
  commit.
