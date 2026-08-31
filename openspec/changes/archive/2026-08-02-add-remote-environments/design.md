## Context

`remote.json` holds the outputs of one deployed CDK stack — the start/stop/deploy
Lambda URLs, the region, and (since the base-URL change) the endpoint address. It
is per-user (each user deploys their own stack) and per-instance (a user may run
several). Today a Spinloop's `REMOTE` resolves to a single file: one beside the
Spinloop (`remoteConfigPath`, `cmd/spinloop/remote.go`) or, when no Spinloop names one,
the single per-user `~/.config/spinloop/remote.json` (`internal/remote.ConfigPath`).
That single fallback clobbers across projects and offers nowhere to keep more
than one instance, and a committed `REMOTE ./remote.json` bakes one user's
deployment state into a shared repo.

This change makes the Spinloop carry a stable *name* and keeps the *state* in a
per-user registry of named environments. It is the foundation
`add-remote-bootstrap` builds on — bootstrap writes a deployed instance into the
registry instead of a shared file.

## Goals / Non-Goals

**Goals:**

- Multiple remote instances per user, each its own environment, none clobbering.
- Spinloops stay committable: only an environment *name* is in the file; all
  deployment state is per-user.
- Backward compatible: existing `REMOTE ./remote.json` (path) usage is unchanged.
- `spinloop remote ls` to see the registered environments.

**Non-Goals:**

- Deploying or destroying instances (that is `add-remote-bootstrap` and
  `pnpm cdk`). This change only concerns how a Spinloop resolves to a
  deployment's control config and how those configs are stored and listed.
- Solving whether the CDK can host several instances in one AWS account (stack
  naming/topology) — noted as a downstream concern for the bootstrap change.
- A remove command (`spinloop remote rm <name>`) — flagged as a likely follow-up;
  removing a registry entry would not tear down the AWS stack.

## Decisions

### An environment is a directory, not a config-map entry

Each environment is `${XDG_CONFIG_HOME:-~/.config}/spinloop/remotes/<name>/` with a
canonical `remote.json` inside. Unlike the alias registry (name→path entries in
`config.json`), environments are filesystem-native: `remote.json` is a whole
document a deployment produces, so a directory per environment is the natural
home, listing is a directory scan, and bootstrap can drop the file straight in.
The directory (rather than a flat `<name>.json`) leaves room for other
per-environment state later (e.g. a saved `.env` or deploy config). New helpers
in `internal/remote` beside `ConfigPath`: `EnvDir(name)` →
`.../spinloop/remotes/<name>`, `EnvConfigPath(name)` →
`.../remotes/<name>/remote.json`, and a lister over `.../spinloop/remotes/`.

### `REMOTE` value: name vs path disambiguation

A value that is a **bare name** — no path separator and no `.json` suffix —
resolves to `EnvConfigPath(name)`. A value that **looks like a path** — has a
separator, is absolute, or ends in `.json` — resolves as a file as today. This
rule is applied in both places a `REMOTE` value is read: `resolveRemoteConfig`
(control commands) and `remoteBaseURL` (apply's base-URL lookup), so the two
never diverge. The rule is deliberately simple and matches how the alias registry
already distinguishes a name from a path (`config.go` rejects path-like alias
names).

### The `default` environment replaces the single-file fallback

Where discovery used to fall back to `~/.config/spinloop/remote.json`, it now uses
the `default` environment (`.../remotes/default/remote.json`). This keeps the
"works from anywhere with no Spinloop" convenience inside the one model, so there is
no special single file living outside the registry.

### Migration of an existing `~/.config/spinloop/remote.json`

For users who already have the old single file: on read, if
`.../remotes/default/remote.json` is absent but the legacy
`.../spinloop/remote.json` exists, treat the legacy file as the `default`
environment (read-through), and document moving it to
`.../remotes/default/remote.json`. No silent rewrite. This keeps existing setups
working without a flag day.

### `spinloop remote ls`

A new `cmdRemoteList` scans `.../spinloop/remotes/`, reads each `remote.json`, and
prints name, base URL and region, marking entries whose `remote.json` is missing
or unreadable (mirroring how `spinloop alias --list` marks a missing target). It
contacts no endpoint. Empty registry prints a plain "no environments" line.

### Interaction with `add-remote-bootstrap`

Both changes modify `remote-endpoint`'s "Remote command group" requirement (this
one adds `ls`, bootstrap adds `bootstrap`). They are sequenced — this change
lands first — so bootstrap's delta will be rebased to include `ls` in its
baseline. `concord overlap` will flag the shared requirement until then; that is
expected for sequenced changes, not a genuine conflict.

## Risks / Trade-offs

- **Ambiguous `REMOTE` values** (a name that happens to end in `.json`, or a
  bare word the user meant as a relative file) → the separator/`.json` rule is
  documented, and a name that looks like a path is treated as a path (never
  silently invented as an environment).
- **Legacy single file** → read-through migration keeps it working; documented
  path to move it, no destructive rewrite.
- **`default` magic name** → mildly implicit, but preserves today's no-Spinloop
  convenience; users with several instances simply name them and rely on the
  Spinloop rather than the default.
- **Registry drift from real stacks** → `ls` reflects what is on disk, not what
  is deployed in AWS; a stale entry is a listing concern, and teardown/removal is
  a separate (flagged) follow-up.

## Open Questions

- Whether to add `spinloop remote rm <name>` now or as a follow-up (recommended
  follow-up; it must be clear it removes only the local registry entry and does
  not destroy the AWS stack). This pairs with a future `spinloop remote destroy`
  (the inverse of bootstrap) which would tear down the AWS infra *and* remove the
  registry entry — see the `add-remote-bootstrap` design's Future work.
- Whether `spinloop show` should also surface the environment list (as it does
  aliases) — deferred unless wanted.
