## 1. Tile draw

- [x] 1.1 In `cmd/outfit/dashboard_render.go`, make the in-flight case of `dashTileContent` render the node's last completed refresh beside the verb and the action's lines: the state line (only when non-empty) and the report body, and only when the last result is an OK answer
- [x] 1.2 Factor the report body — serving line, last-active line, resource bars and token counters — into shared helpers used by both the settled and in-flight cases, keeping the settled tile's rule that the resource block shows only for a running node and the settled tile byte-for-byte unchanged

## 2. Tests

- [x] 2.1 Add byte-stable in-flight tile cases under the pinned colour profile: a measuring report (state, serving, bars), an early-boot report (state and serving only), and a failed round (the tile keeps the verb and the action's lines alone)
- [x] 2.2 Add a model-level case: a cloud round lands while a start is in flight, and its report shows on the tile beside the action's lines

## 3. Docs and checks

- [x] 3.1 Update the doc comments on `dashTileContent` and `beginAction`, and the in-flight-tile clause in AGENTS.md, to the beside-not-replacing behaviour
- [x] 3.2 Run `gofmt`, `go vet ./...` and `go test ./...` and keep package coverage at or above 80%
