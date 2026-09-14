## REMOVED Requirements

### Requirement: The fleet harness command

**Reason**: `spinloop fleet harness` is a second spelling of `spinloop code`
(and `spinloop harness open`, which it shortens). The two already end in the
same two calls, and `code` does strictly more: it forwards trailing arguments
to the launched agent, takes `--providers`, and takes `--env`. The one
behaviour `fleet harness` had that `code` lacked — launching with no Spinloop
when the fleet names a gateway — is a fact about routing rather than about
which command was typed, and moves to the `fleet-routing` capability, where it
now applies to every launch routed through a fleet.

Keeping both would leave two commands for one job in a CLI being reorganised
to have one. This one resolves by deleting rather than elevating, because
`harness` and `code` are already top level.

**Migration**: `spinloop fleet harness` becomes `spinloop code --fleet <path>`
(or `spinloop harness open --fleet <path>`). Typing the old spelling fails
naming the replacement, as a moved top-level command does.

The flags carry over unchanged: `-O`/`--spinloop`, `--node`, `--prefer`,
`--no-wake`, `--wake-timeout` and `-H`/`--harness` all mean what they meant,
and `--fleet`/`-f` still names the fleet file.

One difference is deliberate: `fleet harness` defaulted the fleet file to the
`fleet.yaml` beside it in every case, while a launch picks one up only when the
worn Spinloop was not named explicitly. A launch that named its Spinloop and
relied on the old always-default now passes `--fleet` to say so. The rule that
goes is the one that existed because the fleet came from the command rather
than from beside the Spinloop — a distinction with only one command left to
make it.
