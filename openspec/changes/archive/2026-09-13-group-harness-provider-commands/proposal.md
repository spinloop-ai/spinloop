## Why

The top level carries 17 commands, and the ones that manage a harness's config
(`add`, `remove`, `apply`, `unapply`, `show`, `export`) and the ones that inspect
the provider catalogue (`list`, `init-providers`) sit at the same level as the
engine and fleet commands. [Issue #200](https://github.com/spinloop-ai/spinloop/issues/200)
asks to group them: the config verbs under `harness` — which is already the tool's
most-used entry point — and the catalogue commands under a new `provider` group.
That shrinks the top level to a stable set and puts each command where its subject
already lives.

Grouping the config verbs under `harness` makes `harness` a group, so the launch
can no longer sit on a bare `spinloop harness`. [Issue #203](https://github.com/spinloop-ai/spinloop/issues/203)
asks for a dedicated launch spelling; this change takes its first proposal —
`spinloop harness open` — and defers its second (a top-level `code` shortcut) to a
later change.

## What Changes

- **BREAKING**: `add`, `remove`, `apply`, `unapply`, `show`, and `export` are no
  longer top-level commands. Each becomes a subcommand of `harness`
  (`spinloop harness add`, …), carrying the same flags, arguments and output it
  has today.
- **BREAKING**: the launch moves from a bare `spinloop harness` to
  `spinloop harness open`, which carries the same flags, the leading-Spinloop
  rule, and the forwarding behaviour a bare `harness` had. `spinloop harness`
  becomes a plain command group — like `fleet` and `remote` — so a bare
  `spinloop harness` (or a first word that is not a subcommand) shows its help
  and launches nothing. This removes the last special case that treated
  `harness` as a command: the first word after it is now always a subcommand.
- **BREAKING**: `--get`/`--set` move from `harness` to a new `harness config`
  subcommand (`spinloop harness config --get` / `--set <name>`); a bare
  `spinloop harness config` reports the active harness, as `--get` did.
- **BREAKING**: `list` and `init-providers` are no longer top-level. A new
  `provider` group holds both (`spinloop provider list`, `spinloop provider
  init`), with the same flags, arguments and output; `init-providers` is
  renamed `init`, since its parent already says provider.
- **BREAKING**: the old top-level spellings are removed with no deprecation
  window, matching the precedent of the `REMOTE` keyword removal. Running one
  fails with an error that names the new home (e.g. an error giving
  `spinloop harness add`), not the generic unknown-command message.
- `alias` and `unalias` stay top-level, as do `serve`, `up`, `daemon`, `gateway`,
  `hf`, `fleet`, and `remote`.
- No command's behaviour changes beyond its spelling and the `harness`
  re-grouping: `open` launches exactly as a bare `harness` did, and `config`
  reports and stores exactly as `--get`/`--set` did.
- Documentation, examples, and the root help text follow the new spellings.

## Capabilities

### New Capabilities

(None — every behaviour change lands in an existing capability.)

### Modified Capabilities

- `harness-management`: a new requirement defines the `harness` command group —
  a plain group whose subcommands are the six config verbs, `open` (the launch),
  and `config` (report/store the default harness) — and the failure of the old
  top-level spellings; "Stored harness preference" moves to `harness config`,
  "Launching the harness" and "Applying a Spinloop on launch" move to
  `spinloop harness open` (the subcommand-dispatch/collision rule drops, since
  `open` has no subcommands), and "Showing the configured state" moves to
  `spinloop harness show`.
- `provider-catalog`: "Catalogue listing" and "Catalogue scaffolding" move to
  `spinloop provider list` / `spinloop provider init`, and a new requirement
  defines the `provider` group and the failure of the old top-level spellings.
- `cli-ux`: "An error names what to do about it" gains the scenario of a command
  invoked at a spelling that moved.
- `shell-completion`: the completion surface follows the new tree; the `harness`
  group's first slot offers only its subcommands, `open`'s first slot completes a
  Spinloop, and `config --set` offers harness names.
- `spinloop-files`, `provider-selection`, `alias-registry`, `model-discovery`,
  `opencode-integration`, `pi-integration`, `lucinate-integration`,
  `huggingface-spinloops`, `local-serving`, `remote-spinloop-sources`:
  requirement wording updated to the new invocation spellings; no behaviour
  change.

## Impact

- `cmd/spinloop`: `newRootCmd` stops registering the eight commands at the top
  level; `harnessCmd` becomes a plain group (its six config subcommands plus a
  new `openCmd` for the launch and a `configCmd` for `--get`/`--set`); a new
  `providerCmd` group (the same `groupFallback` bare behaviour as `fleet` and
  `remote`) holds `list` and `init` (the old `init-providers`, renamed); the
  root's Args validator maps the eight old spellings to errors naming the new
  home; the root's long description and the moved commands' help text follow.
- `docs/` and `examples/`: the moved commands' pages merge into the `harness`
  and `provider` command pages, and every reference to the old spellings is
  updated; `README.md` follows.
- `internal/` is untouched.
