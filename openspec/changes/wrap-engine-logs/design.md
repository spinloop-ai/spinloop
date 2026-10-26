# Design

## Summary

Both surfaces already share the three-section frame and the clipper
(`dashClip`); wrapping follows the same pattern — one shared width-aware
wrapper, a per-view wrap flag, and row-based slicing of the pane at draw
time. Nothing about the tailing, budgets, or polling changes: wrapping is a
rendering concern, applied between "lines we retain" and "rows we draw".

## Decisions

### The toggle key is `w`, in both views

Neither view answers to `w` today (serve: arrows/pgup/pgdown/`f`/`q`;
detail: `esc`/`f`/`s`/`k`/`a`/`x`), it is mnemonic, and one key for one
behaviour across surfaces matches how `f follow` already works. The footer
gains `w wrap` in both views, in the same `key meaning` shape as its
neighbours, and the header's log-state area gains the wrap state beside the
follow state — `log: following   wrap: on` — so the current mode is visible
without pressing keys, the same transparency the paused state gives.

### Wrapping is hard character wrapping, not word wrapping

Log lines are data, not prose: a JSON blob or a llama.cpp prompt line may
carry no spaces to break on at all, and breaking on spaces would leave
ragged short rows and mis-align columns a reader may be scanning. A hard
cut at exactly the pane width, terminal-style, loses nothing and keeps every
continuation row full. Continuation rows start at the left margin with no
marker or indent — the terminal's own `tail` behaviour is what operators
already read.

### A shared `dashWrapLines` beside `dashClip`

`cmd/spinloop/dashboard_render.go` gains the wrapper (it already owns the
clipping that keeps a panel the shape of the frame). It returns a line's
rows for a given width: one row when it fits or the width is not exceeded,
`ceil(width/paneWidth)` rows otherwise. Rows are cut at exact *display*
columns — wide characters count two — via `ansi.StringWidth` and repeated
`ansi.CutWc(line, 0, n)` for the head plus a rune-walking remainder, rather
than `CutWc(line, left, right)` with a non-zero left: in the pinned
`charmbracelet/x/ansi` the WcWidth path mishandles `left`, and log content
is plain text anyway (the daemon's capture strips escapes before it reaches
the file), so only display width, not ANSI state, has to survive the cut.
The fast path — the line fits in the pane, which is every line on a normal
terminal — costs one width call.

### Retention stays line-based; only the pane counts rows

`logContent` budgets (`serveViewLogBudget`, `detailLogCapacity`) stay
"lines retained" and the poll/trim code is untouched. At draw time a new
shared step expands the window's lines to rows — each line via `dashWrapLines`
when wrapping is on, via `dashClip` (one row) when off — and takes the last
`avail` **rows** for the pane. With wrapping off this is byte-identical to
today (one row per line), so no existing behaviour shifts.

### Scrolling stays line-based, so wrapped lines move whole

The serve view's `logBehind` counts log lines, not rows: the window's last
visible line is `lines[len-1-logBehind]`, the retained prefix up to it is
expanded to rows, and the pane shows the last `avail` of those rows. Up or
down therefore moves whole wrapped lines in and out of the pane, which is
what "scroll one line" must mean here; the topmost row shown may be a
continuation fragment, exactly as on a terminal running `tail`. `clampBehind`
keeps its current formula (`lines − paneRows`) — unchanged semantics when
wrapped off, and a conservative, pane-full bound when wrapped on. The detail
view has no scroll; its tail-slice simply counts rows now.

### Metrics lines keep clipping

The issue is about engine logs. Metrics and bar lines are fixed-shape panel
content — `dashClip`'s own comment explains why they must not wrap — and the
grid is untouched.

## Risks

- **Per-frame wrap cost**: only the pane's worth is expanded — the window's
  lines, capped by `avail` rows once sliced from the bottom — every poll
  tick (3s), so even a pane of 200-column wrapped rows is trivial next to
  the terminal repaint itself.
- **Tests that pin the clip**: `TestDashDetailViewTruncatesOverflowingLogToAvailableRows`
  and any serve-view test feeding over-width lines now assert wrapping by
  default; those tests get updated to say so, plus new tests for the `w`
  toggle and header state on both views.
