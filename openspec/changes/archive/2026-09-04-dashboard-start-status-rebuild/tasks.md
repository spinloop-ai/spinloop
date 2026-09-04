## 1. The start phase

- [x] 1.1 Add `StartPhaseKind`, `StartPhase` and `RenderPhase(p, now)` to `internal/fleet`, with `RenderPhase` a pure function of the phase and the time
- [x] 1.2 Change `ProgressStarter` to `StartWithProgress(ctx, report func(StartPhase))`, and update `remoteNode.Start` to pass a no-op reporter
- [x] 1.3 Map `remote.Start`'s `progress`/`onState` pair onto phases in `remoteNode.StartWithProgress`: `StateInFlight` → attempting, `no-capacity` + the 503's retry-after → waiting-for-capacity with its due time, a held request → booting, a dropped connection → reconnecting. Leave `internal/remote` untouched
- [x] 1.4 Unit-test `RenderPhase` against a fixed clock: the capacity wait counts down, the boot counts up, and no rendering carries a number that was fixed when the phase was built

## 2. Recording when each reading was taken

- [x] 2.1 Add `At time.Time` to `fleet.NodeResult` and set it in `internal/fleet/fanout.go` as each read returns
- [x] 2.2 Replace the dashboard's `fastGen`/`slowGen` drop rule with a simpler one: show a reading only when it was taken later than the one on screen
- [x] 2.3 Test the case the counters miss: a round that starts before an action finishes and lands after it does not repaint the node's older report

## 3. Old readings, and how often a node is read

- [x] 3.1 Draw a node's age on its tile once its newest reading is older than three of its kind's intervals, and drop it to the unknown health tier
- [x] 3.2 Read a node with an action in flight on the short interval whatever its kind, returning to its kind's cadence once the action settles
- [x] 3.3 Test that a remote node is read more often for the duration of a start and returns to its own cadence afterwards, and that a node with an old reading greys out and recovers when it answers again

## 4. One function produces a tile's contents

- [x] 4.1 Merge `dashNodeContentLines` and `dashHealthTierFor` into one function taking the phase, the reading, how old it is and the current time, returning the tile's lines and its health colour
- [x] 4.2 Write one test listing every phase against every state a reading can be in — no reading yet, fresh answer, stale answer, failed round — including a start waiting for capacity next to a reading that says the node is running, which is the case that caused the original bug
- [x] 4.3 Keep the settled (no action in flight) tile byte-for-byte unchanged, pinned by the existing tile tests. Superseded during implementation: the operator asked for a full-width coloured header row on every tile, which changes the settled tile's first line. The existing tile tests were updated to that header and still pin every tile byte for byte; the rest of a settled tile is unchanged.

## 5. Shared wording

- [x] 5.1 Rewrite `cmd/spinloop`'s `startProgress` as a renderer over the same phase stream, dropping its own state field and heartbeat wording
- [x] 5.2 Update the CLI tests that pin `remote start`'s stderr lines, and confirm nothing on the eval-able stdout path changes

## 6. Docs and checks

- [x] 6.1 Update the dashboard pointers in `AGENTS.md`, and note the phase contract in `docs/internals.md` if it is not obvious from the code
- [x] 6.2 Run `gofmt`, `go vet ./...` and `go test ./... -cover`, keeping total coverage at or above 80%

## 7. The tile's own appearance

Asked for during implementation, and specified alongside the rest.

- [x] 7.1 Draw a braille spinner beside an in-flight verb, its frame chosen when the tile is drawn, and repaint the board on a fast tick while any action is in flight
- [x] 7.2 Draw a tile's first line as a header bar: light text on one neutral background across the tile's full width, with the health glyph keeping its own colour on top, so the bar never competes with the glyph for what a node's health is read from
- [x] 7.3 Give the board a title bar of its own on the same surface, shared by the grid and the detail view: the product in the brand accent, the screen beside it, the fleet file and the screen's details to the right
- [x] 7.4 Draw the selected panel's border in the brand accent rather than the amber it shared with the attention health tier, and keep the accent off anything that reports a node's state
- [x] 7.5 Draw each key-help entry as a key and what it does: the key in the terminal's own text colour, its meaning in the muted ink, leaving the status line and a confirmation's question as prose
