## Why

A fleet client polls its nodes every few seconds, and at the default `info`
level every one of those successful requests writes a line — on a machine with
several nodes, the daemon's log is mostly polling. Successful request summaries
carry no information an operator needs unless they are hunting a specific
request, so they belong one level down: `debug`, alongside the engine command
line, which is today the only other debug record.

## What Changes

- **BREAKING** (for anything that greps the default log): a summary of a
  successfully served API request is recorded at `debug` severity instead of
  `info`. With the default threshold of `info`, successful request summaries no
  longer appear at all; `--log-level debug` (or `SPINLOOP_LOG_LEVEL=debug`)
  brings them back.
- Rejected requests (4xx) stay at `warn` and server-side failures (5xx) stay at
  `error` — the grading that silences routine traffic first and failures last is
  unchanged, and the `--log-level warn` recipe for a polled node keeps working,
  just becomes unnecessary for the default.
- The default threshold stays `info`; the level control, its precedence
  (`--log-level` flag > `SPINLOOP_LOG_LEVEL` > default), and the fail-fast on an
  unrecognised level are all unchanged.
- Docs that describe what each level shows are updated to match: the level
  table in `docs/commands/serve.md` and its fleet-polling note,
  `docs/env-vars.md`, `docs/README.md`, and `docs/http-api.md`.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `api-logging`: the severity a successful request summary carries moves from
  informational to debug; the "summaries appear by default" expectation in the
  level-control requirement changes with it.

## Impact

- `internal/daemon/logging.go` — `levelForStatus`'s success branch.
- Tests in `internal/daemon/logging_test.go` and `cmd/spinloop/logging_test.go`
  that assert summaries at `info`.
- User-facing docs listed above. No HTTP API change: statuses, headers, and
  bodies are exactly as before at every level.
