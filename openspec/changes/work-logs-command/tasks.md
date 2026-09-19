## 1. A single read

- [ ] 1.1 Add `workLogsCmd` (`work logs <id>`), `--url`/token flags via
  `workAPIFlags`, calling `GET /v1/items/{id}/log` via `workRequest` and
  printing the log field — verify with a test asserting the printed output
  matches a fake API's answer
- [ ] 1.2 An item with no output yet (`log: null`) prints nothing and does
  not fail — verify with a test against a fake API answering `log: null`
- [ ] 1.3 An id the API refuses (404, unknown id) fails the command naming
  the id, the way `abort`/`remove` already report a refusal — verify with
  a test against a fake API answering the refusal
- [ ] 1.4 Wire `workLogsCmd` into `workCmd`'s subcommands and register
  `--url` completion the way the other subcommands do — verify with
  `spinloop work logs --help` listing it and `spinloop work --help`
  listing `logs` among the subcommands

## 2. Following

- [ ] 2.1 Add `-f`/`--follow` to `workLogsCmd`; unset, behavior is
  unchanged from task 1 — verify with a test asserting the flag defaults
  false and a single read still happens
- [ ] 2.2 Add `workLogsInterval` (1s, a package var) and
  `followWorkLogsLoop`: each tick, `GET /v1/items/{id}/log`, printing only
  the new suffix when the new answer still starts with what was already
  printed, the whole answer otherwise — verify with a test driving the
  loop over a sequence of fake answers, asserting only the new bytes are
  printed each time
- [ ] 2.3 Each tick also reads the item's `state` from `GET /v1/items`
  (decoded as `[]orchestrator.ItemView`, matching `work list`'s own
  decode); the loop keeps polling through `backlog`/`running` and stops
  once the state is `done`/`failed`, after one final log poll — verify
  with a test asserting the loop keeps polling while state is
  `backlog`/`running`, printing nothing new once no more arrives, and
  ends the first tick it sees `done`/`failed`, having already printed that
  tick's log
- [ ] 2.4 An item that drops out of the list mid-follow (removed) ends the
  follow the same way an ended state does, without failing — verify with
  a test whose fake API stops carrying the id partway through
- [ ] 2.5 Wire `-f` through `followUntilInterrupted`, matching `fleet logs
  -f`/`remote logs -f`'s own wiring: an interrupt ends the follow cleanly,
  not as a failure — verify with a test cancelling the loop's context
  directly (the same pattern `follow_test.go` already uses for the shared
  helper) and asserting a nil return

## 3. Tests and the suite

- [ ] 3.1 Run the full suite — verify with `go test ./... -race -cover`
  passing, no regressions, `cmd/spinloop` coverage holding at or above 80%
