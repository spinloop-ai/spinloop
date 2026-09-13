# Design: group harness and provider commands

## Context

`cmd/spinloop` builds a fresh Cobra tree per invocation (`newRootCmd`,
`cmd/spinloop/commands.go:25`). Today `harness` is a leaf command
(`commands.go:99`) that launches the active harness: it has
`DisableFlagParsing: true`, `cobra.ArbitraryArgs`, a hand-rolled flag set
parsed in its `RunE`, and a `ValidArgsFunction` (`harnessSlot`,
`cmd/spinloop/complete.go:273`) because the engine cannot see past
flag parsing. It also carries `--get`/`--set` for the active harness.
`add`/`remove`/`apply`/`unapply`/`show`/`export` are top-level leaves built
in `cmd/spinloop/main.go`; `list` and `init-providers` are too. `fleet` and
`remote` are already groups whose bare form shows help via `groupFallback`
(`commands.go:320`).

The move is a command-tree change: no handler logic moves, no `internal/`
package changes. The whole problem is in the tree: where the eight
commands hang, what a bare/unknown word under `harness` means, and what
the eight old top-level spellings do.

## Goals / Non-Goals

Goals:

- The six config verbs reachable only as `spinloop harness <sub>`, and the
  two catalogue commands only as `spinloop provider <sub>`, behaviour-
  identical to today.
- `spinloop harness open` launches exactly as a bare `spinloop harness`
  does today, including all of the leading-Spinloop, `--`, and
  `--spinloop` rules.
- `spinloop harness` is a plain command group — like `fleet` and `remote` —
  so a bare `spinloop harness`, and any first word that is not a
  subcommand, shows its help and launches nothing. This drops the last
  special case that treated `harness` as a command: the first word after it
  is now always a subcommand.
- `spinloop harness config` reports (`--get`, the default) and stores
  (`--set <name>`) the default harness, as `--get`/`--set` did on `harness`.
- The eight old top-level spellings fail with an error that names the new
  home, per the `cli-ux` error requirement.
- Completion follows the tree, as it already does.

Non-goals:

- No change to any command's flags, output, or exit codes beyond the
  re-grouping described above.
- A top-level `code` shortcut (issue #203's second proposal) is deferred to
  a later change.
- `alias`/`unalias` and every other top-level command stay put.
- No deprecation window or compatibility shims (decided with the user;
  matches the `remove-remote-keyword` precedent).

## Decisions

### 1. `harness` becomes a plain group; `open` takes the launch

Rather than keeping `harness`'s `RunE` and dispatching subcommands around it
(Cobra allows a parent with both, but it keeps `harness` a hybrid that
special-cases its own first word), `harness` is made a plain group exactly
like `fleet` and `remote`: `Use: "harness"`, `RunE: groupFallback`, and the
launch moves to a new `open` subcommand. A bare `spinloop harness` shows the
group's help; `spinloop harness run --continue` reports `run` as an unknown
subcommand instead of launching.

The new `openCmd` carries the launch body verbatim from the old `harness`:
`DisableFlagParsing: true`, `cobra.ArbitraryArgs`, the same hand-rolled flag
set parsed in its `RunE`, and the same `ValidArgsFunction` (renamed
`launchSlot`). Because `open` is a leaf with no subcommands, the first word
after it is either a Spinloop name (applied, then launch) or a forwarded
argument — the old "does the leading word name a subcommand?" collision rule
drops out entirely. `--get`/`--set` move to a sibling `configCmd`
(`--get` is the default when no flag is given; `--set <name>` stores).

The six config subcommands (`addCmd`, `removeCmd`, `applyCmd`, `unapplyCmd`,
`showCmd`, `exportCmd`) are added to the group unchanged; they register the
same selection flags, so `spinloop harness -H pi add` still resolves to `add`
with `-H pi` — handled by the parent's flag set, as `groupFallback` groups
already do.

Alternative considered: keeping `harness` as a hybrid (its `RunE` launches
when the first word is not a subcommand). Rejected — it is precisely the
"treat `harness` like a command" complication this change exists to remove,
and it forces a subcommand-vs-launch disambiguation on every `harness`
invocation. `open` is a short, unambiguous spelling for "launch the
harness".

### 2. `provider` is a plain group

A new `providerCmd` mirroring `fleetCmd`/`remoteCmd`: `Use: "provider"`,
`RunE: groupFallback` (bare shows the group's own help, an unknown word
gets Cobra's error), children `listCmd()` and `initProviderCmd()` — the old
`init-providers`, renamed `init` under the group that already says
provider.

### 3. Old spellings map to errors that name the new home

Cobra's default for a missing top-level word is `unknown command "add" for
"spinloop"` plus did-you-mean suggestions — which cannot suggest
`harness add`, because suggestions are scoped to the root's children. The
check is Cobra's `legacyArgs` fallback, applied inside `Find` only while a
command's `Args` field is nil; Cobra v1.10 has no hook into it, and
intercepting the resulting error in `main` would mean matching a message
format the library does not promise to keep.

Instead the root takes a custom `Args` validator plus a help-only `RunE`,
and both are needed: `execute` returns help for a non-runnable command
before it validates args, so a runnable root is what gets the validator
reached at all, and a non-nil `Args` is what stops `legacyArgs` raising
the generic error first. The validator maps the eight removed names to
their replacements and returns the `cli-ux`-shaped error; every other word
gets the same error `legacyArgs` produced, suggestions included, via the
exported `SuggestionsFor`.

Alternative considered: registering the old names as hidden stub commands
whose `RunE` returns the message. Rejected — a stub stays in the tree, so
`__complete add <TAB>` would complete the flags of a command that cannot
run; the validator keeps the tree exactly as the new spec describes it.

Error wording (final copy is implementation, shape is fixed by `cli-ux`):
lowercase, no trailing full stop, gives the new spelling — e.g. for
`spinloop add`: `"add" moved: run spinloop harness add`, and for
`spinloop init-providers`: `"init-providers" moved: run spinloop provider init`.

### 4. Completion needs no new table

Subcommand names complete from the tree automatically, which the existing
guard test (`TestCompletionCoversTree`, per the `shell-completion` spec)
enforces. Two existing behaviours do the rest:

- `spinloop provider <TAB>` and `spinloop harness <TAB>`: a group with no
  `ValidArgsFunction` → subcommands only, no file paths. The guard test's
  one exception for a hybrid `harness` launch slot is gone; `harness` now
  behaves exactly like `provider`.
- `spinloop harness open <TAB>`: `open` is a leaf with a `ValidArgsFunction`
  (`launchSlot`), so its first slot completes the Spinloop names and paths a
  launch accepts, and stops offering them once a Spinloop is on the line.
- `spinloop harness config --set <TAB>`: `config` registers harness-name
  completion for `--set` and `--harness` via `compRegister`.

`launchSlot` (formerly `harnessSlot`) is otherwise untouched. The
`harness` command's `Short`/`Long` text is rewritten to describe the group
in the `cli-ux` imperative form; the six moved subcommands' help texts are
carried over unchanged, `open`'s and `config`'s are new, and the root's long
description (which currently points at `spinloop list` and `spinloop show`)
follows the new spellings.

### 5. Test seams and suite

The suite drives commands through per-command seams (`cmdAdd`, `cmdList`,
…, `main.go:1655`). Those seams keep working unchanged because they call
the builders directly; the delta is in tests that dispatch through the
root — root-level tests for the moved commands switch to the new spellings,
and the launch/get-set tests that used to drive a bare `spinloop harness`
switch to `spinloop harness open` / `spinloop harness config`. New
root-level tests cover: each old spelling's error naming its new home, a
genuinely unknown top-level word's unchanged error, bare `spinloop harness`
and `spinloop provider` showing group help (launching nothing), and
`spinloop harness <sub>` dispatching to the subcommand.

## Risks / Trade-offs

- [Scripts pinning a bare `spinloop harness` launch break on upgrade] →
  intentional; the error now names the launch's new home, and the release
  note carries the mapping. The launch is still one word away
  (`spinloop harness open`).
- [Scripts pinning the old top-level spellings break on upgrade] →
  intentional (no deprecation window, per the user); the error names the
  fix, and the release note should carry the eight-name mapping.
- [`open` with `DisableFlagParsing` must handle `-h`/`--help` itself] →
  Cobra only intercepts the help flag when it parses flags, and `open` turns
  that off; pflag still registers the flag (Cobra adds it before `RunE`) and
  would otherwise consume it and launch. `open`'s body checks for it after
  parsing and shows `open`'s own help; a `--` before it puts the flag in the
  forwarded args, so the harness still gets its own help.
- [`help` output changes shape] → the `harness` help now lists
  subcommands; that is the point of the change, and `cli-ux`'s help
  requirement still applies to the new `Short`.

## Migration Plan

Single binary release; no state migrates (the alias registry and stored
harness preference are untouched). Rollback is reverting the release —
there is no on-disk format to undo. Docs and examples ship in the same
release as the move, so no version exists where the docs describe
spellings the binary does not have.

## Open Questions

None — the scope questions (old spellings' fate, `init-providers`'s place,
the `init-providers` → `provider init` rename) were answered before this
change was created, and moving the launch to `open` and `--get`/`--set` to
`config` was confirmed with the user as implementation began.
