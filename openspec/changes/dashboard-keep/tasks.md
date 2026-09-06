# Tasks: dashboard keep

## 1. The deadline rides the stats read (Go)

- [x] 1.1 Add a `RetainUntil` field to `metrics.Stats`, omitted when empty,
      beside `LastActiveAt` and `Ready` — verify with `go build ./...` and the
      mapping test in 1.3.
- [x] 1.2 Add a `RetainUntil` field to `remote.StatsResponse`, matching the
      Lambda's JSON — verify with a decode test in `internal/remote` that a
      reply carrying the field lands on it and one without leaves it empty.
- [x] 1.3 Carry the deadline across the node's stats mapping
      (`statsFromRemote`) — verify with `internal/fleet` tests: a stats reply
      with a deadline ends up on `metrics.Stats`, and one without leaves it
      empty.

## 2. The stats Lambda reports the deadline while it is active

- [x] 2.1 Include the instance's parsed Retain-Until tag in the stats reply
      only while it is a time in the future — verify with vitest cases in
      `remote/test` (a future tag is present, a passed tag is absent, an
      untagged instance is absent) and `pnpm test` passing in `remote/`.

## 3. A node capability for keep

- [x] 3.1 Add the keep capability interface — take a duration, return the
      deadline the control plane set — beside the progress-reporting start
      interface in `internal/fleet` — verify with `go build ./...`.
- [x] 3.2 Implement the capability on the remote node over the existing
      `remote.Keep` call — verify with `internal/fleet` tests: a keep sends
      the set-keep command with the now-plus-duration deadline and returns the
      control plane's value, and a config without the update URL surfaces the
      control plane's named error.
- [x] 3.3 Confirm the local daemon node does not implement the capability —
      verify by a test asserting the dashboard's type assertion fails for it
      (the same test file that covers the progress-start assertion).

## 4. The shared deadline line

- [x] 4.1 Merge the relative keep onto the shared active-figure line in the
      bar-format body — `active  … ago  keep for …` — omitted when the read
      carries neither — verify with render tests: the line's wording when a
      keep is present, and its omission when the field is empty.
- [x] 4.2 Wire the line into the one-shot `fleet metrics` bar format, the
      `remote metrics` bar format, and the `remote metrics` table format when
      present — verify with the existing render tests for those surfaces
      extended for the line.

## 5. The dashboard keep action

- [x] 5.1 Add the prompt state to the dashboard model — a flag and a duration
      buffer — opened by the keep key from the grid and the detail view,
      pre-filled with `4h`, handled before the grid/detail dispatch so
      navigation, selection and refreshes stand still while it is open —
      verify with model tests: the prompt opens on the key for a remote node,
      drives nothing for a local node, and navigation keys do nothing while it
      is open.
- [x] 5.2 Handle the prompt's keys — append, backspace, confirm, cancel — and
      leave the prompt open with the parse reason in the footer when a confirm
      is not a positive duration — verify with model tests over valid entries
      (`4h`, `90m`, `1h30m`) and invalid ones (`4hours`, empty).
- [x] 5.3 Run a confirmed keep through the per-node action machinery — verb
      `keep`, the tile's spinner and elapsed time, one action per node, the
      node re-read immediately on completion — and carry the deadline on the
      action's completion message — verify with model tests: a keep in flight
      shows on the tile, a busy node takes no second keep, and completion
      clears the action and schedules the re-read.
- [x] 5.4 Render the footer outcome for a finished keep — success shows the
      deadline the control plane set, failure shows its reason, the dashboard
      stays open — verify with tests over the line's wording, including the
      no-update-URL failure.
- [x] 5.5 Make the abort key drive nothing on a keep in flight — verify with a
      model test alongside the existing stop-in-flight case.
- [x] 5.6 Show the keep entry in the key help only for a node that supports
      keep and has no action in flight, on both the grid and the detail
      footer — verify with tests: a local node hides it, a busy remote node
      hides it, an idle remote node shows it.
- [x] 5.7 Draw the deadline line on the tile and the detail screen from the
      read, omitted when the read carries none — verify with tile render
      tests: a read with a deadline shows the line on the tile, a read without
      one shows no line, and the detail screen draws the same line.
- [x] 5.8 Update the `fleet dashboard` command's long help to name the keep
      key — verify with `go run ./cmd/spinloop fleet dashboard --help`
      printing the key alongside the existing ones.

## 6. Docs

- [x] 6.1 Update `docs/commands/fleet.md` for the keep key, the duration
      prompt, and the deadline line on panels — verify by reading the section
      against the implemented keys and lines.

## 7. Final checks

- [x] 7.1 `go test ./... -cover` at 80% total or above, `go vet ./...`, and
      `gofmt -l` clean — verify all three pass.
- [x] 7.2 The `remote/` suite (`pnpm test`) passes with the new stats reply
      field — verify the run is green.
- [x] 7.3 `openspec validate dashboard-keep` passes — verify the command
      reports the change valid.
