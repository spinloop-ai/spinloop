# Tasks

## 1. Shared wrapper

- [ ] 1.1 Add `dashWrapLines(line string, width int) []string` beside
      `dashClip` in `cmd/spinloop/dashboard_render.go`: hard wrap at exact
      display columns (wide runes count two), one row when the line fits,
      plain-text fast path; document why it exists beside the clipper's
      "cut, not a wrap" comment.
- [ ] 1.2 Unit-test the wrapper: fits-as-one-row, exact multiple, remainder
      row, wide characters, empty line, width < 1 guard.

## 2. Serve view

- [ ] 2.1 Add a `logWrap bool` to `serveView`, defaulting on in
      `runServeView`; read `w` in `updateKey` to toggle it.
- [ ] 2.2 In `View`, expand the window's lines to rows — wrapped rows when
      on, `dashClip` single rows when off — and take the last `avail` rows;
      show `wrap: on/off` beside the log state in the header.
- [ ] 2.3 Add `w wrap` to `serveViewKeys` footer help.
- [ ] 2.4 Tests: long line wraps across rows and is drawn in full; `w`
      toggles to clipping and back; header shows the state; scrolling moves
      whole wrapped lines; existing clip-shaped tests updated to the wrapped
      default.

## 3. Dashboard detail view

- [ ] 3.1 Add a `detailLogWrap bool` to `dashModel` (set in `openDetail`);
      read `w` in `updateDetailKey`.
- [ ] 3.2 In `detailView`, expand log lines to rows the same shared way,
      slice to the pane's rows, and show the wrap state in the header for
      every node, standing ones included.
- [ ] 3.3 Add `w wrap` to `detailKeys`.
- [ ] 3.4 Tests: wrapping default, `w` toggle, header state, long line in
      full; update `TestDashDetailViewTruncatesOverflowingLogToAvailableRows`
      for the wrapped default.

## 4. Gate

- [ ] 4.1 `gofmt`, `go vet`, `go test ./...` green with coverage ≥ 80%.
