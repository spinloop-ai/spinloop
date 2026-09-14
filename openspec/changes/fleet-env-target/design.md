## Context

See proposal.md — Why. The implementation-relevant current state:

- `Config.NewNode` already builds a cloud node from nothing but a name:
  `remote.EnvConfigPath(entry.Name)`, `remote.LoadConfigFile`,
  `NewRemoteNode` (`internal/fleet/node.go:129-139`). `--env` resolves the
  same two lines (`cmd/spinloop/remote.go:126-142`). The registry key and the
  node name are the same string by construction — `fleet deploy` registers an
  environment under the node's name, which is why the other fleet commands can
  look one up by name at all.
- `Config.Token`/`resolveTokenEnv` return `""` immediately when the node names
  no token variable (`internal/fleet/config.go:593-596`), and a cloud node
  names none. The `.env` beside `Config.Dir` is therefore never read for one,
  so a config with an empty `Dir` is safe for this kind. Engine tokens
  (`EngineToken`, `RemoteEngineToken`) are read only by select, wake, start
  and the gateway — never by the read fan-out.
- `Config.validate` is unexported and runs inside `Load`. It requires at least
  one node, unique names, a known kind, and defaults an empty kind to daemon.
  A config built in memory bypasses it.
- Each fleet command calls `fleet.Resolve(path)` itself, so there is no
  single place a second way of naming a target could be added.
- The launch path already holds the exclusivity rule, in
  `applyRoutedSpinloop` (`cmd/spinloop/main.go`), with wording inherited from
  the old `REMOTE`/`FLEET` check.

## Goals / Non-Goals

**Goals:**

- One function answers "what am I acting on", for every fleet command and for
  the launch.
- A registered environment is addressable by every fleet command, with no
  file on disk.
- The node contract, the fan-out and the renderers are untouched: this change
  supplies a different `*fleet.Config`, not a different way of using one.

**Non-Goals:**

- No change to the `remote` group. Its read commands keep their own rendering
  until the change that moves the read verbs up retires them.
- No capability interfaces (`Coster`, `SourceLogger`). `fleet metrics` has no
  `--cost` and `fleet logs` no `--source`, so nothing here needs them; they
  belong to the change that unifies the read verbs, where `remote metrics
  --cost` has to survive the collapse.
- No `--env` on the mutating fleet commands beyond what falls out of the
  shared resolver — `fleet start`/`stop`/`deploy` accept the flag because
  every fleet command does, and `deploy` against a one-node target is
  equivalent to `remote deploy --env`. No new semantics are invented for
  them here.
- No `--env` on `fleet harness`, and no change to it at all — see D6.
- No synthesised multi-environment fleet ("every registered environment as
  one fleet"). It is a plausible next step; it is not this change.

## Decisions

**D1: The resolver returns a `*fleet.Config`, not a new target type.**

A one-node cloud fleet is a fleet, so the thing every command already accepts
is the thing the resolver returns. Introducing a `Target` abstraction over
"one environment" and "a fleet file" would add a layer whose only job is to
collapse back into a `*fleet.Config` at every call site.

Alternative: a `Target` interface anticipating a third kind — a gateway
target, where the fleet is read through one endpoint rather than fanned out
(the shape `spinloop orchestrator` already uses). Rejected for now, but the
resolver is a function returning a config from flags, so introducing a target
type later changes the resolver and its callers and nothing else. A fleet
can have no gateway, so fan-out stays the primary path regardless.

**D2: The one-node config is built by a constructor in `internal/fleet`, and
validated the same way a parsed one is.**

`fleet.ForEnvironment(name)` (or similar) returns the config, running the same
validation `Load` runs so an empty or path-shaped name fails in one place
rather than at each call site. Keeping it in `internal/fleet` also keeps
`Config`'s fields unexported-by-convention knowledge — which kinds default,
what `validate` enforces — where the type lives.

Alternative: have each command assemble a `fleet.Config{Nodes: …}` literal
(rejected — four call sites constructing an unvalidated config, and the
kind-defaulting rule copied into `cmd`).

**D3: `Path` and `Dir` stay empty, and nothing reads them for this kind.**

`Dir` feeds the adjacent-`.env` lookup, which a cloud node never reaches
(see Context). `Path` appears in error messages and in `fleet route`'s
"Routing through %s" line. Rather than inventing a fake path like
`<env:prod>`, anything that reports which fleet it acted on names the
environment. A fake path would be a string that looks openable and is not.

Risk accepted: a future fleet-wide setting read unconditionally from `Dir`
would silently see the process's working directory. D2's shared constructor is
where a guard goes if that arises.

**D4: The exclusivity check moves into the resolver, wording unchanged.**

The rule and its sentence already exist on the launch path; this moves them
rather than writing a second version. Both paths then fail identically, which
is the point — an operator learns the rule once.

Note the asymmetry the spec states explicitly: `--env` beside a *directory's*
`fleet.yaml` is not a conflict, only `--env` beside an explicit `--fleet` is.
A file that happens to be in the working directory is not a statement of
intent; a flag is. This matches how the launch path already treats an
explicitly named Spinloop versus a discovered one.

**D5: `--env` takes no short form on the fleet commands.**

`-e` is `--env`'s short form on `harness`, `apply` and `unapply`, but those
are single-target commands where it is the only such flag. Across the fleet
group `-f` already means two things depending on subcommand, which is the
confusion this group does not need more of. Long form only, everywhere.

**D6: `fleet harness` does not take `--env`.**

Every other fleet command does, so the exception needs a reason: `fleet
harness` is on its way out, and adding flag surface to a command being
deleted is investment in the wrong place.

`code` (and `harness open`, which it shortens) and `fleet harness` already end
in the same two calls — `applyRoutedSpinloop` then `launchAgent` — and `code`
is a strict superset on three counts: it forwards trailing arguments to the
launched agent, takes `--providers`, and takes `--env`. `fleet harness` has
exactly one behaviour `code` lacks, launching with no Spinloop when the fleet
names a gateway, and `code` already holds the slot for it — the branch that
currently refuses `--fleet` without a Spinloop. Folding that one branch across
makes `fleet harness` deletable, which is a step of the elevation work rather
than a detour from it.

That convergence is its own change: it deletes a command and moves a
launch behaviour, where this one adds a flag and moves nothing. Until then
`fleet harness` keeps working exactly as it does now, and a launch against a
single environment is `code --env <name>`, which configures the harness from
what the environment reports rather than routing to a fleet of one.

## Risks / Trade-offs

- [Two ways to read one environment — `remote status --env x` and `fleet
  status --env x` — print differently] → deliberate and temporary, and stated
  in the proposal: the duplicate is what the read-verb change deletes. The
  alternative is changing `remote`'s output in a change that is otherwise
  additive.
- [A one-node config silently missing a fleet-wide setting a command expects]
  → the spec states that such a fleet carries none of them and takes the
  defaults, and D3 names where a guard goes; the settings in question
  (preference, wake policy, concurrency) are all meaningful only across
  several nodes anyway.
- [`fleet deploy --env x` overlapping `remote deploy --env x`] → they resolve
  the same environment and derive the same config, so the overlap is a second
  spelling rather than a second behaviour. `fleet deploy`'s node-name-is-the
  environment-key rule is what makes them agree.
- [The flag count on fleet commands grows] → `--env` is one flag replacing the
  need for a file that exists only to hold a name.

## Migration Plan

None. Every existing command line keeps its meaning and its output; the
change only makes previously-invalid ones valid. Rollback is a revert.
