## 1. The work list API client

- [x] 1.1 Add a small client helper in `cmd/spinloop` that takes the API's base
  address and the token, builds a request with the `Authorization: Bearer`
  header, and returns the response status and the API's JSON error message —
  verify with a unit test pointed at an `httptest` server that asserts the
  bearer header is sent and a non-2xx's message is surfaced
- [x] 1.2 Rewrite `work add` to send the item's fields (`id`, `instructions`,
  `dir`, `tags`, `priority`) to `POST /v1/items` through the helper and report
  the API's answer — verify with a test that asserts the POSTed body carries the
  fields and the command says the item is added on a 201, or names the API's
  refusal on a 400/409
- [x] 1.3 Rewrite `work remove <id>` to call `DELETE /v1/items/{id}` — verify
  with a test that asserts the path carries the id and the command names the
  API's refusal for a running item (409) and an absent id (404)
- [x] 1.4 Rewrite `work abort <id>` to call `POST /v1/items/{id}/abort` —
  verify with a test that asserts the path and the command names the API's
  refusal where the item is not running
- [x] 1.5 Rewrite `work list` to read `GET /v1/items` and render each returned
  item with the existing `workListLine`, plain lines off-terminal and coloured
  on it, keeping the `ls` alias — verify with a test that asserts one plain line
  per item in order and that `ls` behaves the same

## 2. Flags and the file logic that goes away

- [x] 2.1 Put `--url` (required), `--api-token`, and `--api-token-file` on the
  `work` group so every subcommand inherits them, resolving the token through
  the existing `daemonToken` helper, and remove the `--items` flag — verify with
  tests that a command with no `--url` fails naming the flag before any request,
  that both token flags at once is a refusal, and that the token falls back to
  the `SPINLOOP_API_TOKEN` environment
- [x] 2.2 Remove the now-unused file helpers and waits from `cmd/spinloop`
  (`loadWorkItems`, `awaitAbort`, `awaitRecordGone`, `workWaitBound`,
  `workWaitTick`) and any `internal/orchestrator` imports that fall out — verify
  with `go build ./...` and `go vet ./...` passing and no remaining references to
  the removed symbols
- [x] 2.3 Confirm the shell completion reflects the new flags and none of the
  generated or hand-written completion references the removed `--items` — verify
  by running the `__complete` path for `work add` and checking it offers `--url`
  and not `--items`

## 3. Tests and the suite

- [x] 3.1 Rewrite `cmd/spinloop/work_test.go` to work the commands against a
  test server standing in for the work list API, covering add, remove, abort,
  list, the token resolution, and each refusal — verify with
  `go test ./cmd/spinloop/ -cover` passing and the package's coverage holding at
  or above 80%
- [x] 3.2 Run the full suite with the race detector — verify with
  `go test ./... -race -cover` passing, no regressions

## 4. Documentation

- [x] 4.1 Rewrite `docs/commands/work.md` for the API-based commands: the
  `--url` flag, the token's sources, and that a refusal reads the way the API
  states it — verify by reading it against the `work-commands` spec's scenarios
- [x] 4.2 Fix the cross-references that describe the work commands as working
  the file (`docs/work-items.md`, `docs/commands/orchestrator.md`) so they point
  at the commands as the work list API's client — verify by reading each
  reference and confirming it no longer implies file-working from the shell
