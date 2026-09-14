## Context

See proposal.md — Why. The implementation-relevant current state:

- Both commands end in `applyRoutedSpinloop(...)` then `launchAgent(...)`.
  `runFleetHarness` calls `launchAgent(h, nil, "", ...)`; the launch calls
  `launchAgent(h, rest, providers, ...)`. The shared body is already one path.
- The behaviour to move is one branch. `runFleetHarness` catches
  `readSpinloop`'s failure, resolves the fleet, and proceeds with
  `spinloop.Selection{Provider: gatewayProviderID}` when the file names a
  gateway (`cmd/spinloop/fleet.go`). The launch has the matching slot already:
  `} else if route.fleetPath != "" { return fmt.Errorf("--fleet needs a
  Spinloop: …") }` (`cmd/spinloop/code.go`).
- `harness open` runs with `DisableFlagParsing: true` and its own
  `splitHarnessArgs`, so spinloop's flags are recognised wherever they appear
  and everything from the first non-flag, non-Spinloop word forwards
  byte-for-byte. `fleet harness` uses ordinary Cobra parsing with
  `cobra.MaximumNArgs(1)`.
- `spinloop code` dispatches to `harness open`'s body rather than
  reimplementing it, and its capability says it accepts the same flags — so
  the change lands in one place and `code` inherits it.
- `movedTopLevelCommands` (`cmd/spinloop/commands.go`) is how the
  harness/provider grouping signposts a command that moved: a map from the old
  spelling to the new one, consulted by the root's `Args` validator.
- `examples/gateway-docker/run-tests.sh` drives `spinloop fleet harness`, so
  the example is a live consumer, not just documentation.

## Goals / Non-Goals

**Goals:**

- One command launches an agent against a fleet.
- The gateway-without-a-Spinloop launch survives the deletion intact — it is
  the only thing `fleet harness` could do that `code` could not.
- The old spelling names its replacement rather than reporting an unknown
  command.

**Non-Goals:**

- No change to what a gateway launch configures: the provider naming, the
  model-list population and the warn-don't-fail behaviour move verbatim.
- No change to node routing, waking, or the gateway itself.
- No new flags. `code` already carries every flag `fleet harness` had.
- No deprecation period. The house pattern for a moved command is a hard break
  that names the replacement, set by the harness/provider grouping.

## Decisions

**D1: The launch absorbs the branch; `fleet harness` is deleted, not aliased.**

A Cobra alias would keep two entries in the tree for one behaviour, which is
what the reorganisation is removing. Deleting and signposting matches how
`add`, `apply`, `show` and the rest were moved under `harness`, so an operator
who learns the pattern once can predict it.

Alternative: keep `fleet harness` as a thin wrapper calling the launch
(rejected — the wrapper is the duplication, just spelled shorter).

**D2: `code` keeps its own implicit-fleet rule; `fleet harness`'s goes.**

`fleet harness` always defaulted to the `fleet.yaml` beside it. A launch picks
one up only when the worn Spinloop was not named explicitly — the rule that an
explicitly named Spinloop travels to its fleet only by flag.

Keeping both is impossible with one command, and `code`'s is the one to keep:
it is the rule every other launch already follows, and the reason for
`fleet harness`'s was that its fleet came from the command rather than from
beside the Spinloop, which is not a distinction a single command can make.

This is the one behaviour change a user could notice. It is stated in the
removed requirement's Migration, and the repair is to pass `--fleet`.

**D3: The moved spelling is signposted through the root's existing mechanism.**

`movedTopLevelCommands` maps a first word to its replacement. `fleet harness`
is not a first word, so the signpost belongs on the `fleet` group rather than
the root: an unknown `fleet` subcommand named `harness` fails with
`"fleet harness" moved: run spinloop code --fleet <path>`. The mechanism is
the same shape; only the command it hangs off differs.

Alternative: leave it to Cobra's unknown-subcommand error (rejected — it would
suggest nothing, and this is the spelling the examples and docs used until
now).

**D4: The gateway helpers move with the behaviour, not into a shared home.**

`fleet harness` calls helpers for the gateway provider id, the model-list fetch
and the provider label. They have exactly one caller after this change, so they
move to the launch path beside the branch that uses them rather than becoming a
package boundary for a single consumer.

## Risks / Trade-offs

- [A launch that named its Spinloop and relied on `fleet harness`'s
  always-default fleet silently stops routing] → the launch already announces
  the fleet it routes through on stderr, so a launch that stops routing says
  less rather than saying something wrong; the Migration names the repair, and
  `examples/gateway-docker` is updated as the worked example of it.
- [The gateway path inherits `DisableFlagParsing` and the custom arg split] →
  the tests move with the behaviour, and a case `fleet harness` never had —
  trailing arguments forwarded to the agent alongside a gateway launch — is
  worth a test of its own, since it is the combination neither command has run
  before.
- [`examples/gateway-docker` is a CI-run integration test, so a missed call
  site fails the build rather than a user] → this is a benefit; the example is
  updated in the same change.

## Migration Plan

`spinloop fleet harness` → `spinloop code --fleet <path>`. The old spelling
fails naming the new one, so a user who types it is told what to type instead.
A launch that named its Spinloop explicitly and relied on the old
always-default fleet adds `--fleet`. Rollback is a revert; nothing is persisted
or transmitted differently.
