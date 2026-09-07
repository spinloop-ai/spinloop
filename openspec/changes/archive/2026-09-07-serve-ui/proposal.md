## Why

`spinloop serve` streams the engine's raw output straight to the terminal: the
operator sitting next to their own engine cannot see how hard it is working —
CPU, RAM, GPU, token counters — and once a log line has scrolled past it is
gone. The fleet dashboard already has a detail screen showing a node's metrics
and tailed log, but the machine where the engine runs in the foreground has
nothing equivalent.

## What Changes

- **BREAKING (interactive runs only):** `spinloop serve` on a terminal opens a
  full-screen view instead of forwarding the engine's stdio: the engine's
  metrics — state, what it serves, last active, the resource series, the token
  counters — above the engine's log, in the three-section layout of the fleet
  dashboard's node detail screen. A run whose output is not a terminal keeps
  today's stdio-forwarded behaviour exactly.
- The view draws every resource series in both formats at once — the gauge of
  the current reading and the bar of the retained history — where the
  dashboard draws one or the other, toggled by `g`.
- The up and down arrow keys scroll the log pane through older output (page up
  and page down by a page); while the pane is on the newest line, new output
  appears as it is written. `f` pauses and resumes the log's follow, as in the
  detail screen.
- Under the view the engine's stdout and stderr are captured to the daemon's
  state-dir engine log — the same file `spinloop daemon` writes — instead of
  being forwarded to the terminal. The view tails that file, and with `--api`
  the control API's log endpoint serves it, so `serve --api`'s log endpoint
  reports the real log rather than "missing".
- `serve --api` shows the same view with the control API listening alongside.
- The view run switches the engine's own metrics endpoint on for an engine
  that has one — the same rule a supervised engine follows — and runs the
  daemon's activity and history sampling for the life of the view, so the bars
  have history to draw. An engine with no metrics endpoint gets the host's
  series, as any node does.
- `q` or Ctrl+C stops the engine and exits; when the engine exits on its own
  the view closes and serve exits with the engine's exit status, as today.

## Capabilities

### New Capabilities

(none — the view is a surface of `spinloop serve`, which `local-serving` owns,
the same way the dashboard's detail view lives in `fleet-client`)

### Modified Capabilities

- `local-serving`: the stdio-forwarded foreground becomes the non-terminal
  case; on a terminal serve runs the view. New requirements for the view
  itself — its terminal gate and layout, both formats per series, the log's
  tail and arrow-key scroll, its keys and exit, and the engine-output capture
  it runs on — and the "Serve basics" and "Control API flag" requirements
  change with it.

## Impact

- `cmd/spinloop/serve.go`, `cmd/spinloop/serve_daemon.go` — the terminal
  check, the view's launch, and the supervised-foreground construction pulled
  out of `serve --api` into one helper the view path and the `--api` path
  share, parameterised by the log target and the API listener.
- A new model/renderer pair in `cmd/spinloop/` for the serve view, reusing the
  fleet detail screen's frame helpers, the daemon's in-process
  status/metrics/log reads, and the shared metrics renderers.
- `cmd/spinloop/metrics_render.go` — a combined renderer drawing each
  series' gauge and bar together, and a width parameter on the gauge.
- `openspec/specs/local-serving/spec.md` — updated by this change's delta.
- `docs/commands/serve.md` — the foreground description and the view's keys.
- No new dependencies: bubbletea and lipgloss are already used by the fleet
  dashboard.
