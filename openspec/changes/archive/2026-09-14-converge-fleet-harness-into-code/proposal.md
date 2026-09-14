## Why

`spinloop fleet harness` and `spinloop code` launch an agent against a fleet,
and they already end in the same two calls — `applyRoutedSpinloop`, then
`launchAgent`. What differs is a thin shell, and it differs in `code`'s favour
on every count but one:

| | `code` / `harness open` | `fleet harness` |
| --- | --- | --- |
| trailing args reach the agent | yes, byte-for-byte | no, dropped |
| `--providers` | yes | no |
| `--env` | yes | no |
| launches with no Spinloop | via `--env` | via the fleet's `gateway:` section |

So `fleet harness` is a second spelling of a command that does strictly more,
minus one behaviour. That one behaviour — a launch through a gateway needs no
Spinloop, because a gateway resolves the model per request rather than by
matching a node against one — is a fact about routing, not about which command
was typed. `code --fleet <path>` refuses it today only because the branch that
would allow it lives in the other command.

Two commands for one job is the thing the CLI reorganisation is removing, and
this one resolves by deleting rather than moving: `harness` and `code` are
already top level, so the fleet-level spelling has nowhere to be elevated to.

## What Changes

- A launch routed through a fleet SHALL no longer require a Spinloop when the
  fleet file names a gateway. `spinloop code --fleet ./fleet.yaml` in a
  directory with no Spinloop configures the harness against the gateway,
  populates its model list from the gateway's `GET /v1/models`, and launches —
  exactly as `spinloop fleet harness` does today.
- The gateway-launch behaviours move with it unchanged: the provider named and
  keyed by the gateway ("Gateway (dev-2)"), the model list populated from the
  live gateway, a failed model query warning rather than failing, and no
  default model set.
- **BREAKING** `spinloop fleet harness` is removed. Typing it fails naming its
  replacement, the way the harness/provider grouping signposts a moved
  command.
- `code` and `harness open` keep their own implicit-fleet rule: an explicitly
  named Spinloop does not pick up a working-directory `fleet.yaml`, and routes
  only when `--fleet` is given. `fleet harness`'s always-default rule goes with
  the command — it existed because its fleet came from the command rather than
  from beside the Spinloop, which stops being a distinction when there is one
  command.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `fleet-routing`: a launch routed through a fleet whose file names a gateway
  no longer needs a Spinloop; the gateway-provider naming and model-list
  population that `fleet harness` carried become part of routing itself.
- `fleet-client`: the fleet harness command is removed, and with it the
  requirement describing it.

## Impact

- `cmd/spinloop/code.go`: the branch that refuses `--fleet` without a Spinloop
  becomes the gateway check `fleet harness` runs today.
- `cmd/spinloop/fleet.go`: `fleetHarnessCmd`, `runFleetHarness` and
  `cmdFleetHarness` are deleted, along with their tests in
  `fleet_harness_test.go`; the gateway helpers they call move to the launch
  path rather than being removed.
- `cmd/spinloop/commands.go`: the subcommand is unregistered, and the moved
  spelling is signposted the way the top-level moves are.
- `docs/commands/fleet.md`, `docs/commands/harness.md`,
  `docs/commands/code.md`, `docs/commands/gateway.md`, and
  `examples/gateway-docker/` — which drives `fleet harness` in its test run —
  name the replacement.
- No change to the fleet file format, the gateway, the environments registry,
  or the control plane.
