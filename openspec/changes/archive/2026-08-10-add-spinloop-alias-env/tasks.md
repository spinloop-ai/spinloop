## 1. Resolution

- [x] 1.1 Add an `SPINLOOP_ALIAS` resolver in `cmd/spinloop/main.go` beside
  `resolveAlias`: read the variable, return early when unset or empty, reject a
  value that is not name-shaped (`config.ValidAliasName`), look it up with
  `config.Load`/`File.Alias`, and `os.Stat` the target. Each failure names
  `SPINLOOP_ALIAS` — unregistered points at `spinloop alias --list`, dangling
  suggests re-pointing with `spinloop alias -n <name> <path>` or `spinloop unalias
  <name>`. No shadowing check.
- [x] 1.2 Call it from `readSpinloop` in the `path == ""` branch, before the
  `./Spinloop` default, and print `Using SPINLOOP_ALIAS "<name>" (<path>)` to
  stderr when it decides the path.
- [x] 1.3 Make `cmdAlias` pass `spinloop.DefaultFile` instead of an empty path
  when it has no positional argument, so registering ignores the variable, and
  say why in a comment.
- [x] 1.4 Extend the "no `Spinloop` found in the current directory" error to
  mention `SPINLOOP_ALIAS` alongside the path and alias it already suggests.
- [x] 1.5 Update the doc comments on `readSpinloop` and `resolveAlias` to cover
  the environment source and why it is not shadowed by a file on disk.

## 2. Tests

- [x] 2.1 Add a `TestMain` to `cmd/spinloop` that unsets `SPINLOOP_ALIAS` before
  running, so a developer's exported value cannot reach the suite.
- [x] 2.2 In `cmd/spinloop/alias_test.go`, cover resolution: the variable
  supplies the Spinloop for `apply` in a directory with no `Spinloop`, and the
  stderr note names the variable, the alias and the path.
- [x] 2.3 Cover precedence: an explicit path argument and an explicit alias
  argument both beat the variable; the variable beats a `./Spinloop` present in
  the working directory.
- [x] 2.4 Cover the no-shadowing rule: a file named the same as the value in
  the working directory does not displace the registry lookup, and no shadowing
  note is printed.
- [x] 2.5 Cover the errors: unset behaves exactly as today, empty is ignored, a
  path-shaped value, an unregistered value and a dangling one each fail with
  `SPINLOOP_ALIAS` named.
- [x] 2.6 Cover the exclusions: `spinloop harness` with no argument and no
  `-O` applies nothing, `spinloop harness -O` applies the variable's Spinloop, and
  `spinloop alias` beside a different `Spinloop` registers the local one.
- [x] 2.7 Cover one non-`apply` caller end to end (`serve --dry-run` or a
  `remote` subcommand against the existing `httptest.Server` harness) to prove
  the choke point reaches every command.

## 3. Documentation

- [x] 3.1 Add `SPINLOOP_ALIAS` to `docs/env-vars.md` and the env-var table in
  `docs/README.md`, stating the precedence and that it is a registry name.
- [x] 3.2 Document it in `docs/commands/alias.md`, and note it on the pages for
  the commands it reaches: `apply.md`, `unapply.md`, `serve.md`, `harness.md`,
  `remote.md`.
- [x] 3.3 Cover it in `README.md` under "Aliases", including that it changes
  which Spinloop is the default rather than whether one is applied.
- [x] 3.4 Update the "Alias registry" paragraph in `AGENTS.md` with the
  environment source and the two rules that differ from an argument (no
  shadowing, `spinloop alias` opted out).

## 4. Verification

- [x] 4.1 `gofmt` the touched files and run `go test ./... -cover`, keeping
  coverage at or above 80%.
- [x] 4.2 Sanity-check by hand in a scratch directory: export `SPINLOOP_ALIAS`,
  run `spinloop apply` and `spinloop serve --dry-run` from an unrelated directory,
  then unset it and confirm the previous behaviour returns.
- [x] 4.3 `openspec validate add-spinloop-alias-env --strict`.

## 5. Follow-up: the existence gate

- [x] 5.1 Fix `resolveRemoteConfig` and `resolveDaemonSpinloop`, which tested for
  a `./Spinloop` on disk before calling `readSpinloop` and so skipped the variable
  entirely — `spinloop remote status` fell through to the per-user default config.
  Both now ask `defaultSpinloopNamed`, which counts `SPINLOOP_ALIAS` too.
- [x] 5.2 Regression tests for both, plus the spec scenario and the `remote.md`
  and `AGENTS.md` notes that say the gate has to count the variable.

## 6. Coverage of the new paths

- [x] 6.1 Cover the claim the existence gate rests on — an unresolvable
  `SPINLOOP_ALIAS` stops `remote` and `daemon` rather than being passed over for
  their fallbacks (the per-user endpoint config, and an idle start).
- [x] 6.2 Cover the fallbacks that must survive: a Spinloop named by the
  variable but carrying no `REMOTE` still yields to the default environment,
  and an exported-but-empty variable does not count as naming a Spinloop.
- [x] 6.3 Cover `spinloopFromEnv`'s registry-read failure, so a corrupt
  `config.json` is reported by a real command rather than swallowed the way
  completion deliberately swallows it.
