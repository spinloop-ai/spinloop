# Tasks

## 1. Spec delta

- [x] 1.1 Modify the "The serve view" requirement in the change's `local-serving` spec delta: the figure between the two halves draws in a fixed-width field, so a reading of one, two or three digits keeps the line the same width and the bar half in the same column — with a scenario for a reading that crosses a digit boundary. Every scenario the current requirement has is kept. The main spec is updated when the change is archived.

## 2. Rendering (cmd/spinloop/metrics_render.go)

- [x] 2.1 Draw the figure in `renderStatCombined` in a fixed-width field — three digits, right-aligned, plus the percent sign (`%3.0f%%`) — and update the function's doc comment to say the bar half's column does not depend on the figure's digit count; verify with the tests below and `go test ./cmd/spinloop/`.

## 3. Tests

- [x] 3.1 Update the combined-format expectations in `cmd/spinloop/metrics_render_test.go` to the padded figure: `TestRenderStatCombined`, `TestRenderStatCombinedMemoryWithoutTotal`, `TestRenderStatCombinedWithoutHistory`, and `TestRenderStatCombinedStoppedEngineDrawsHistoryAlone`; the width assertion now counts the figure's fixed-width field.
- [x] 3.2 Add `TestRenderStatCombinedFigureDoesNotShiftTheBar`: the same history with readings of 0, 7, 42 and 100 each draws the line the same width, with the bar half in the same column.

## 4. Verify

- [x] 4.1 `go test ./...`, `go vet ./...`, and `gofmt -l .` clean, with total coverage at or above 80%.
