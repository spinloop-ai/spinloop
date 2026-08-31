## Why

In `spinloop fleet dashboard`, a running node's uptime is appended to the end of
the serving line (`runner  modelID  (up 2h 0m 0s)`). That line is clipped to
the tile's fixed width, so a long runner or model ID pushes the uptime past
the cutoff and it silently disappears — the operator sees the model but not
how long it's been up. [Issue #127](https://github.com/spinloop-ai/spinloop/issues/127).

## What Changes

- Move a running node's uptime off the serving line and onto the tile's top
  line, next to the node's state, where it fits within the fixed tile width
  regardless of how long the runner/model names are.
- The serving line keeps just the runner and model ID.
- Applies to both a settled tile (`name  state  (up ...)`) and a tile with an
  action in flight, where the state prints on its own line beneath the
  action's status lines (`state  (up ...)`).

## Capabilities

No spec-level requirements change — `fleet-client`'s dashboard requirements
already promise a running node's uptime is shown; this only moves which line
it renders on. This change sets `skip_specs: true`.

## Impact

- `cmd/spinloop/dashboard_render.go`: `dashTileContent`, `dashTileServingLine`
- `cmd/spinloop/fleet_dashboard_test.go`: byte-stable tile fixtures that assert
  exact tile text
