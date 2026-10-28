# Proposal

## Why

The serve view and the dashboard's node detail view draw every log line with
a hard clip at the pane width, so an engine line longer than the terminal —
a stack trace, a request dump, a llama.cpp prompt log — silently loses
everything past the cut ([issue #267](https://github.com/spinloop-ai/spinloop/issues/267)).
For logs, the tail of a long line is usually the interesting part, and
there is no way to reach it from the view.

## What Changes

- Log lines in the serve view's log section and the dashboard detail view's
  log section wrap onto continuation rows instead of clipping, by default.
- A new `w` key toggles wrapping on and off in both views; the wrap state is
  shown beside the log's follow state in the header, and `w wrap` joins the
  footer key help in both views.
- Scrolling and tailing keep working at line granularity: the pane shows the
  window's log lines expanded to their wrapped rows, most recent rows at the
  bottom. Metrics sections are unaffected and stay clipped.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `local-serving` — "The serve view's log" gains wrapped-by-default log
  rows and the `w` wrap toggle (with scenarios); "The serve view's keys and
  exit" names `w` among the keys the view offers.
- `fleet-client` — "The detail view shows the node's metrics and log" gains
  the same wrapped-by-default behaviour and `w` toggle for its log section.

## Impact

- `cmd/spinloop/serve_view.go` — wrap state, `w` key, footer help, row-based
  window slicing.
- `cmd/spinloop/dashboard_detail.go` — same for the detail view; `w` key in
  `updateDetailKey`.
- `cmd/spinloop/dashboard_model.go` — `detailKeys` footer gains `w wrap`.
- `cmd/spinloop/dashboard_render.go` — a shared width-aware line-wrapper
  beside `dashClip`.
- View tests in `cmd/spinloop/serve_view_test.go` and
  `cmd/spinloop/fleet_dashboard_test.go`.

No wire formats, daemon API, or log-file behaviour change — this is entirely
a rendering concern.
