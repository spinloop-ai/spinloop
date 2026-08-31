## Why

spinloop resolves its own config dir from `${XDG_CONFIG_HOME:-~/.config}/spinloop`,
which leans on `$HOME`. Under systemd — where the cloud instance now runs
`spinloop daemon` — a service gets no `$HOME`, so the daemon resolved a *different*
config dir (a relative `.config/spinloop`, i.e. `/.config/spinloop` from `/`) than
the boot script wrote to (`/root/.config/spinloop`). The daemon found no deploy
config and every boot `POST /v1/start` returned "nothing to serve"; `start`
hung against a healthy-but-idle instance. The real fix is not to chase `$HOME`
but to make spinloop's config location explicit and overridable, so a service can
be told exactly where its config lives — and to close the silent-fallback
footgun that let an unresolved home resolve to a bogus relative path.

## What Changes

- **`SPINLOOP_CONFIG_DIR` env var**: when set, it is spinloop's config dir verbatim
  (no `/spinloop` appended). Precedence: `SPINLOOP_CONFIG_DIR` >
  `${XDG_CONFIG_HOME}/spinloop` > `~/.config/spinloop`.
- **One shared resolver**: the two duplicated resolvers today
  (`internal/config.Path` for `config.json`; `internal/remote.ConfigHome` for
  `remote.json`, the `remotes/<name>/` environment registry, the CDK source
  dir, and `daemon.StateDir`) route through a single function, so the override
  and the fallback rules apply everywhere spinloop reads or writes its own state.
- **Loud on an unresolvable home**: with no override and no `XDG_CONFIG_HOME`,
  a failure to resolve `$HOME` is now an error naming `SPINLOOP_CONFIG_DIR`,
  rather than silently yielding a relative `.config/spinloop`.
- **The cloud instance pins its config dir**: the `spinloop-daemon` systemd unit
  sets `SPINLOOP_CONFIG_DIR=/var/lib/spinloop`, and the boot writes the daemon's
  deploy config there. The daemon's state (deploy-config.json, engine.log) no
  longer depends on `$HOME`; the CloudWatch-agent and logrotate paths follow.

Not in scope: the harnesses' own config files (Pi's `~/.pi`, opencode's config)
— those belong to the harness, not spinloop, and keep their own locations.

## Capabilities

### New Capabilities

- `config-location`: how spinloop resolves its own config directory — the
  `SPINLOOP_CONFIG_DIR` override, the precedence over `XDG_CONFIG_HOME` and
  `~/.config`, that every spinloop-owned file (config.json, remote.json, the
  environment registry, the daemon state dir, the CDK source dir) resolves
  under it, and the loud failure when no home can be resolved.

### Modified Capabilities

- `remote-engine-host`: the instance's daemon config dir is pinned via
  `SPINLOOP_CONFIG_DIR` to a fixed system path, so what the boot writes and what
  the daemon reads are the same location regardless of `$HOME`.

## Impact

- `internal/config`: gains the shared `Dir()` resolver (honours
  `SPINLOOP_CONFIG_DIR`, errors on unresolvable home); `Path()` builds on it.
- `internal/remote.ConfigHome`: delegates to `internal/config.Dir()` (no cycle
  — `config` is the stdlib-only leaf package). `daemon.StateDir` inherits the
  override for free.
- `remote/lambda/runners/daemon-boot.ts`: the unit sets
  `SPINLOOP_CONFIG_DIR=/var/lib/spinloop`; the deploy config is written under it.
- `remote/lambda/start/index.ts` + `remote/lib/image-stack.ts`: the engine-log
  path and its baked logrotate config move to `/var/lib/spinloop/daemon/`.
- **Rollout**: the override lives in the spinloop binary, which is baked into the
  AMI — so the cloud fix needs a new spinloop release, an `spinloopVersion` bump
  in `remote/lib/config.ts`, a re-bake, and a Lambda redeploy. Existing local
  installs are unaffected (default path unchanged).
- Test coverage stays >= 80% (`go test ./... -cover`); `remote/` `pnpm test`.
