## Why

Every Spinloop today has to live on the machine running `spinloop`: `readSpinloop`
only ever calls `os.Stat`/`os.ReadFile`, so sharing one means committing it to
a repo the recipient must clone, or pasting it by hand. Teams that want to
publish a house-standard Spinloop — "here's the sanctioned way to point your
agent at our gateway" — have no way to hand out a single link, and there is no
way to register a short name for such a link the way `spinloop alias` already
does for a local file.

`PRESET` and `REMOTE` have the same limitation one level down: both are
resolved relative to the Spinloop's own directory, which implicitly assumes the
Spinloop sits on local disk. Nothing in `internal/spinloop`, `serve.go`, or
`remote.go` can fetch anything over the network.

## What Changes

- **A Spinloop path may be an `http://`/`https://` URL.** Every command that
  resolves a Spinloop path today — `apply`, `unapply`, `serve`, `alias`,
  `harness --spinloop`, and the `remote` subcommands — goes through the single
  `readSpinloop` chokepoint in `cmd/spinloop/main.go`; that function learns to
  fetch a URL argument instead of reading local disk, with the same
  directory-style convenience a local path gets (a URL ending in `/` gets
  `Spinloop` appended).
- **`spinloop alias` can register a URL.** `spinloop alias -n <name> <url>` stores
  the URL verbatim in the registry (`config.json`'s `aliases` map already
  holds plain strings, so no schema change is needed), and it resolves
  afterwards exactly like a local-path alias: `spinloop apply <name>`,
  `spinloop serve <name>`, etc. Listing (`spinloop alias --list`, `spinloop show`)
  never probes a URL-valued alias over the network — the "(missing)" liveness
  check stays local-path-only, since checking it would mean a network call on
  every listing.
- **`PRESET` and `REMOTE` fetch lazily, at pull time.** A relative `PRESET` or
  `REMOTE` value resolves against the Spinloop's own source — a local directory
  join when the Spinloop came from disk (unchanged), URL-relative resolution
  when it came from a URL. Either may also be given as an absolute URL
  regardless of where the Spinloop itself lives. Crucially, nothing about
  parsing or resolving a Spinloop fetches these files: the bytes are only
  pulled at the point a command actually consumes them (`spinloop serve` and
  `spinloop remote deploy` for `PRESET`; the `remote` subcommands and `apply`'s
  base-URL fallback for a path-form `REMOTE`) — exactly the laziness the local
  case already has, now preserved when the target is remote instead of
  assumed away.
- **A URL-sourced Spinloop has no adjacent `.env`.** The `.env`-beside-the-Spinloop
  lookup the `remote` commands perform is inherently a local-filesystem idea;
  for a URL-sourced Spinloop it is skipped rather than attempted against a
  nonsense path, leaving the Spinloop's own `ENV` instructions and the process
  environment as the two remaining, already-specified sources.

## Capabilities

### New Capabilities

- `remote-spinloop-sources`: the shared mechanism for resolving and fetching an
  Spinloop-family reference (the Spinloop itself, a `PRESET`, or a path-form
  `REMOTE`) that may be a local path or an `http(s)` URL — relative-reference
  resolution, on-demand (not eager) fetching, and the safety bounds (timeout,
  response size cap, status handling) placed on a remote fetch.

### Modified Capabilities

- `spinloop-files`: "Spinloop path resolution" additionally accepts an `http(s)`
  URL wherever a local path or alias is accepted today.
- `alias-registry`: registering, resolving, and listing an alias additionally
  cover a URL-valued target, including the listing-time liveness-check
  carve-out for URLs.
- `local-serving`: preset-based serving accepts a `PRESET` that is itself a
  URL, or relative to a URL-sourced Spinloop, fetched only when `serve` builds
  the launch command.
- `remote-endpoint`: the `REMOTE` configuration-discovery requirement accepts
  a path-form `REMOTE` that is itself a URL, or relative to a URL-sourced
  Spinloop, fetched only when a `remote` subcommand resolves it.
- `remote-local-environment`: a URL-sourced Spinloop has no adjacent `.env` to
  load; the local-environment load falls back to the Spinloop's own `ENV`
  instructions and the process environment alone.

## Impact

- **New code**: `internal/spinloopsrc` — `IsURL`, `Resolve` (relative-reference
  joining across path/URL sources), and `Fetch` (local read or bounded HTTP
  GET), shared by every call site below.
- **Changed code**: `cmd/spinloop/main.go` (`readSpinloop`, `resolveAlias`,
  `cmdAlias`'s path-to-registry-value step); `cmd/spinloop/serve.go`
  (`resolvePresetPath` and its `os.ReadFile` call); `cmd/spinloop/remote.go`
  (`resolveRemotePath`, `remoteConfig`, `resolveRemoteConfigForSpinloop`,
  `applySpinloopEnv`, and `deployConfigFor`'s preset read); `internal/remote`
  (an exported bytes-based entry point so a URL-sourced `remote.json` reuses
  `LoadConfigFile`'s env-override and validation logic instead of duplicating
  it).
- **Docs**: `docs/spinloop-file.md` (URL sources for the Spinloop path, `PRESET`,
  and `REMOTE`), `docs/commands/alias.md` (aliasing a URL).
- **New example**: `examples/remote-spinloop/` (an `Spinloop` + `preset.ini`
  written to be hosted behind a URL, with a `README.md` walking through
  `spinloop apply <url>`, registering and reusing a URL alias, and `spinloop
  serve` lazily fetching the preset), matching the existing
  `examples/<name>/{Spinloop,README.md}` convention.
- **No breaking changes**: every existing local-path, directory, and
  registered-alias flow is untouched; a URL is a new kind of value these
  arguments accept, not a change to how an existing one is interpreted.
