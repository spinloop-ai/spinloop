## Context

See proposal.md — Why. The implementation-relevant current state:

- `resolveRemoteConfig(envName, spinloopArg)` (`cmd/spinloop/remote.go`) reads
  the Spinloop's `ENV` instructions, then branches: a named environment is
  `EnvConfigPath` + `LoadConfigFile`; no name falls to `LoadDefault`. Nine
  subcommands call it — `start`, `stop`, `pause`, `restart`, `status`,
  `metrics`, `logs`, `env`, `keep`. `deploy` does not: it already requires the
  flag.
- `LoadDefault` tries `remotes/default/remote.json`, then the superseded
  `~/.config/spinloop/remote.json`, then falls through to
  `finishConfig(Config{}, …)` — which is what makes an overrides-only
  configuration work at all.
- `finishConfig` applies the `SPINLOOP_REMOTE_*` overrides and raises the
  "remote is not configured: paste … into %s" failure, taking the source path
  for the message.
- `Config.Environment` is set only by unmarshalling a file. Its own comment
  says the shared Lambdas "reject a call without one", so an overrides-only
  configuration reaches the control plane with no identifier.
- `LoadConfig` and `ConfigPath` exist but have no caller outside the test
  suite once `LoadDefault`'s loop goes.

## Goals / Non-Goals

**Goals:**

- No command changes the state of a cloud instance without being told which
  one.
- One resolution rule for every environment name.
- The documented "no `remote.json` on disk" workflow keeps working, and starts
  carrying the identifier it always needed.

**Non-Goals:**

- No stored "current environment" to make the flag optional again. That is the
  implicit target under another name, and it is what this removes. If typing
  the flag proves tiresome, the answer is a shell alias or a fleet file, both
  of which already exist.
- No change to the registry layout, the fleet file, the control plane, or
  `deploy`, which already requires the flag.

## Decisions

**D1: The flag is required on every subcommand, not only the mutating ones.**

The danger is sharpest for `start`, `stop`, `pause`, `restart` and `keep`, and
a narrower change could require the flag only there. Against that: a rule with
an exception list is one an operator has to remember, and the reads are how the
mutations get typed — someone who runs `remote status` and then `remote stop`
in the same shell should not have the second mean something the first did not
warn them about. One rule, no list.

Alternative: required for mutations, optional for reads (rejected — two rules,
and the read is the rehearsal for the write).

**D2: `default` stays a legal name and loses only its privilege.**

Removing the name as well as the fallback would be banning a string for no
safety gain: the risk was never the name, it was that a command assumed it.
`--env default` resolves like `--env prod`, and an existing `remotes/default/`
directory keeps working.

**D3: A named environment with no file may be configured by the overrides, and
the name becomes the identifier.**

The documented environment-variable workflow reached the control plane with an
empty `Config.Environment` — a latent bug, since the identifier is what selects
the instance. Requiring the flag supplies the missing piece for free: the name
the user typed is exactly the identifier that was absent.

So `LoadConfigFile` takes the environment name, and where the file is absent
but the overrides are complete it returns a config carrying that name. This is
strictly more capable than today, in the one case that was quietly broken.

Alternative: drop the overrides-only path with the default environment
(rejected — it is documented, it is how CI configures the commands, and the
bug it carried is fixed by this change rather than deepened by it).

**D4: `LoadDefault`, `LoadConfig` and `ConfigPath` are deleted, not deprecated.**

With no fallback there is no "the environment when none is named", so the
function that expressed it goes rather than lingering as a synonym for
`LoadConfigFile("default")`. `LoadConfig` and `ConfigPath` have no callers left.
Deleting rather than leaving them means a missed call site is a compile error.

## Risks / Trade-offs

- [A command that used to run bare now fails] → that is the change. The
  failure names the flag and lists the registered environments, so the next
  thing to type is in the error.
- [One environment still means typing the flag every time] → the cost of the
  guarantee, and the reason D2 keeps the name short. A fleet file remains the
  way to drive several targets without repeating yourself.
- [Requiring the flag on reads is stricter than the danger warrants] → D1's
  trade, taken deliberately: one rule an operator learns once beats a list of
  which commands are safe bare.
- [The overrides-only path changes shape] → it gains a required flag and a
  correct identifier; the variables themselves are untouched, and a scenario
  covers it.

## Migration Plan

None needed. A command that ran bare takes `--env <name>`; a configuration at
the superseded path is moved into `remotes/<name>/` or re-created. Rollback is
a revert, and nothing is written, so no state needs undoing.
