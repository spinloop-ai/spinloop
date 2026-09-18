## Context

`Dispatcher.Launch` (`internal/orchestrator/dispatch.go`) today does five
things in one method: checks the item's directory, resolves the harness's
one-shot form and the node's model, resolves the catalogue's
`openai-compatible` provider, calls `harness.Harness.Apply(...)` — which
deep-merges a provider block into the harness's own host config file,
`ConfigPath()` resolved from `$HOME`/`XDG_CONFIG_HOME` — and finally starts
the process (`d.start`, normally `startChild`, a thin `exec.Command`
wrapper). The result satisfies `Child` (`Wait`/`Stop`/`Kill`), which
`pass()` and `Abort()` hold onto without caring how it runs.

`pass()`'s failure handling already treats a `Launch` error as one item's
problem: it records the item failed, naming the error, and goes on to the
rest of the backlog (`internal/orchestrator/worklist.go`). Nothing above
`Launch` needs to change for a new backend to slot in, provided a backend's
own failures surface the same way — as an `error` from `Launch`.

`harness.Harness.Apply` is built to merge into an existing file: it loads
whatever is there, adds or updates one provider's block, and writes the
whole thing back, preserving everything else the operator's own host
config carries. That is the wrong tool for a docker launch's config: the
scoped, per-launch directory starts empty, and mounting the host's real
config into a container to solve it would work against the isolation the
whole change is for.

## Goals / Non-Goals

**Goals:**
- A `Launcher` interface behind which `Dispatcher.Launch`'s five steps
  split into a shared plan (directory, form, provider, selection) and a
  backend-specific execution (host process vs. container)
- The bare backend's observable behavior is unchanged — same process
  group, same host config file, same env
- The docker backend: host networking, the two bind mounts, a scoped
  config carrying one provider, the official image
- `docker` unreachable is caught once, at startup, when `--dispatch
  docker` is given — not per item
- `harness.yaml`'s `env` reaches a launch under either backend the same
  way; its `startup`/`shutdown` scripts bracket the harness under either
  backend, and shutdown's "always runs, even aborted" guarantee holds
  without `Abort`, `pass`, or the `Child` interface changing shape

**Non-Goals:** see proposal.md; restated only where it shapes a decision
below.

## Decisions

### The interface sits at `Launch`, not at `exec.Command`

`d.start` is already a seam (`func(bin string, args []string, dir,
logPath string, env []string) (Child, error)`), but it is too narrow: the
docker backend does not run a bin/args/dir/env tuple the host way — it
needs to build mounts and choose an image, and it does not call
`harness.Apply` against the host at all. The seam moves up to the whole
`Launch(item Item, node Node, logPath string) (Child, error)` call.

```go
type Launcher interface {
    Launch(item Item, node Node, logPath string) (Child, error)
}
```

`Dispatcher` becomes the thing that resolves the shared plan and hands it
to one of two launchers (`bareLauncher`, `dockerLauncher`), chosen once at
construction from `--dispatch`. `WorkList` keeps holding a `Launcher` where
it holds a `*Dispatcher` today; the field's type is the only call-site
change outside `internal/orchestrator/dispatch*.go`.

### The shared plan

Both backends need: the item's directory (checked/created the way
`Launch` already does), the harness's one-shot form and args
(`oneShot[d.h.Name()]`), and the resolved `spinloop.Selection` (provider
key, model, the gateway's base URL, display name). This becomes one
function, `resolvePlan`, called by both launchers before they diverge —
extracted from today's `Launch` body verbatim, no behavior change.

### Rendering a provider block without touching the host

`harness.Harness` gains one method, implemented by opencode and Pi (the
two harnesses with a one-shot form; lucinate has neither):

```go
// RenderProviderConfig returns a fresh config file's bytes, carrying only
// this one provider — no merge with anything already on disk. It is what
// Apply's merge starts from when the file does not yet exist, factored out
// so a caller can have the content without writing it to ConfigPath().
RenderProviderConfig(p *catalog.Provider, sel spinloop.Selection, resolve func(string) string) ([]byte, error)
```

`Apply` is refactored to call it for the "file does not exist yet" case it
already has, rather than duplicating the provider-block construction. The
docker launcher calls it directly, writes the bytes to a fresh temp
directory (one per launch, named for the item), and mounts that directory
in — never the host's real `ConfigPath()`.

### Where the config mount lands inside the container

The official image fixes `HOME=/home/agent`. Each dispatchable harness's
config path is `$HOME`-relative today (opencode:
`.config/opencode/opencode.json`, Pi: `.pi/agent/models.json`), so the
docker launcher carries a small table of those suffixes — parallel to,
and no bigger than, the existing `oneShot` map — and mounts the rendered
file at `/home/agent/<suffix>`. This needs nothing from the harness or
catalogue beyond what `RenderProviderConfig` already returns; a harness
added to `oneShot` later needs one line here, the same as it needs one
line there.

### Mounts and network

- `<item.Dir>` → a fixed in-container workspace path (e.g. `/workspace`);
  the harness's command runs with that as its working directory, and the
  container's `WORKDIR` matches
- The rendered config directory → `/home/agent/<suffix>`'s parent, read
  only from the harness's perspective after the one write
- The default bridge network, not `--network host` — see "A loopback
  gateway reaches the container by host.docker.internal, not host
  networking" below for why
- The token reaches the container as an environment variable
  (`-e <name>=<value>`), the same variable `Apply`'s `resolve` would have
  read from the host's own environment for a bare launch

### A loopback gateway reaches the container by host.docker.internal, not host networking

The original design used `--network host` on the theory that it puts the
container on the same network the orchestrator itself is, so a
loopback-bound gateway just works. That holds on native Linux docker, but
not reliably anywhere else: Docker Desktop's Mac and Windows builds run
containers inside a VM, and `--network host` only reaches the real host's
loopback when an operator has turned on a specific, off-by-default Docker
Desktop setting. Without it, `localhost`/`127.0.0.1` inside the container
is the VM's own loopback — not the machine running spinloop, and not
reachable at all. A `fleet.yaml` naming `http://localhost:4000` — the
common case for a gateway run on the same machine — hits exactly this.

The fix does not touch the fleet file: `dockerReachableGateway` rewrites
the address the *container's own rendered config* points its inference
at, substituting `host.docker.internal` for a `localhost`/`127.0.0.1`/`::1`
host, leaving a routable address (anything else) unchanged. This runs on
a copy of `dispatchConfig` local to the docker launcher's `Launch` — the
bare backend, sharing the same `resolvePlan`, keeps the real host address
it actually needs. `host.docker.internal` resolves on Docker Desktop out
of the box; `--add-host host.docker.internal:host-gateway` on the `docker
run` invocation makes it resolve on native Linux docker too, where it is
not automatic — a no-op where Docker Desktop already provides it.

Dropping `--network host` also undoes the one deliberate trade-off the
original design named (see Risks/Trade-offs below): the container is back
on its own network namespace, so the isolation this whole change is for
is no longer given up for the network dimension specifically. A service
on the host's loopback *other than the gateway* is no longer transparently
reachable as plain `localhost` from inside the container — an item that
needs one reaches it via `host.docker.internal` itself, the same way the
gateway now does; that is a real, narrower trade than before, not a
regression from `--network host`'s own unreliability.

### Running and stopping the container

Shells out to the `docker` CLI (`docker run`, `docker stop`, `docker
kill`, `docker wait`), the way `startChild` shells out to `exec.Command` —
consistent with the codebase's existing preference for thin process
wrappers over vendoring a client SDK (the only real external dependency
elsewhere is `aws-sdk-go-v2`, scoped to `internal/remote`). `docker run
-d` returns the container id; `dockerChild.Wait` is `docker wait`;
`Stop` is `docker stop --time 0` (an immediate `SIGTERM`, no separate
Docker-side grace — the orchestrator's own `stopGrace` already bounds the
wait between `Stop` and `Kill` in `worklist.go`, and a second, stacked
grace inside `docker stop` would just make an abort take longer for no
benefit); `Kill` is `docker kill`.

### `--dispatch docker` checks `docker` once, at startup

The way `HasOneShotForm` fails the command before any item is worked
where the chosen harness cannot run one-shot, `--dispatch docker` runs
`docker version` once when the command starts (not per item): an
unreachable daemon is refused there, naming the fix, rather than failing
the first item and leaving the rest to fail the same way one at a time.

### Image tag follows the spinloop version

The docker launcher's default image reference is
`ghcr.io/spinloop-ai/agent:<spinloop's own version>` (the same version
`main.version` already carries), so a given spinloop binary defaults to
the image built alongside it. An `--dispatch-image` flag overrides it —
useful for testing an unreleased image, or pinning one — deferred to
tasks.md as a small addition once the default path works.

### `harness.yaml`: one loader, read once at startup

A small struct (`Dispatch, Startup, Shutdown string`, `Env
map[string]string`), loaded once when the command starts — the way the
items file and the fleet file already are — from `--harness-config`, or
`harness.yaml` beside the items file, or neither. `resolvePlan` folds
`Env` into the launch's environment for every item; an entry naming the
token's own variable is refused there, before any item runs, the way an
unrecognised `--dispatch` value already is. A missing file is not an
error; a present-but-unparsable one is, the way a malformed items file
already is.

`Dispatch`'s precedence against `--dispatch` is resolved with the same
`cmd.Flags().Changed("dispatch")` check `orchestratorListenAddr` already
uses for `--listen`/`--loopback`: an explicit flag beats the file outright,
so `harness.yaml` never has to know whether the flag's value it might be
overridden by is the flag's own default or a real choice — the command
already tracks that distinction, and `HarnessConfig` does not need to.

### The wrapper: how shutdown gets to always run

Neither backend's `Child` is naturally in a position to run a second
script *after* an abort's `Stop`/`Kill` — those act directly on the
harness's own process (group) or container. Guaranteeing shutdown runs
regardless of why the harness stopped means the thing `Stop`/`Kill` act on
has to be able to run shutdown itself once the harness underneath it is
done, not the orchestrator racing to run it separately after `Wait`
returns.

Both backends launch a small generated shell script instead of the
harness directly, whenever `harness.yaml` names a startup or shutdown
script (never, otherwise — the common case still launches the harness
directly exactly as today, no wrapper, no behavior change):

```sh
#!/bin/sh
set -u
term() { [ -n "${child:-}" ] && kill -TERM "$child" 2>/dev/null; }
trap term TERM INT

sh -c "$STARTUP" ; startup_rc=$?
if [ "$startup_rc" -ne 0 ]; then
    sh -c "$SHUTDOWN"
    exit 97   # sentinel: startup failed, the harness never ran
fi

"$@" & child=$!
wait "$child"; harness_rc=$?

sh -c "$SHUTDOWN"
exit "$harness_rc"
```

(`sh -c ""` is a no-op where a script is absent, so the same wrapper
serves startup-only, shutdown-only, and both.) The wrapper is what
`Stop`/`Kill` now act on — its own process group for bare, the container
for docker — so a `SIGTERM` reaches it first: it forwards the signal to
the harness child, waits for that child the same as the clean-finish path
does, then still runs shutdown before the wrapper itself exits. `Abort`'s
existing wait on `fl.done` therefore already covers shutdown; nothing
about `stopGrace`'s bound, or where `Abort` waits, needs to change — it is
now bounding a slightly longer sequence inside the same process.

`Wait`'s exit code 97 is how `resolvePlan`'s caller tells a startup
failure apart from the harness's own: it fails the item naming the
startup script, rather than whatever the harness itself would have meant
by exiting 97 — a harness's own exit code never reaches this path where
there is no wrapper (no startup/shutdown named), so the sentinel only
matters, and only ever appears, when the wrapper is the one running.

Startup's and shutdown's own stdout/stderr go to the same `logPath` the
harness's already does — the wrapper never redirects internally; whatever
opens `logPath` for the wrapper process (bare: `startChild`'s existing
redirect; docker: the container's own stdout, already what `docker run`'s
log capture points at) catches all three in the order they actually
wrote, with no separate plumbing.

A failing shutdown script does not change the item's own recorded
outcome — that is the harness's exit code (or the sentinel), not
shutdown's — but is visible in the kept log the way shutdown's output
always is, so a broken teardown is not silent.

### gh in the image, opencode and Pi's own tool config untouched

`gh` is installed alongside opencode and Pi in the Dockerfile
(task 4.1); it needs no config of its own to be useful for the common
cases (`gh pr create` against the public API), and `GH_TOKEN` is exactly
the kind of thing `harness.yaml`'s `env` now exists for an operator to
set without a new image.

## Risks / Trade-offs

- **A host-loopback service other than the gateway is no longer plain
  `localhost` from inside the container**, now that `--network host` is
  gone — see "A loopback gateway reaches the container by
  host.docker.internal, not host networking" above. An item that needs
  one reaches it via `host.docker.internal` itself.
- **A second image to build, publish and version.** Mitigated by tying
  its tag to spinloop's own version, so there is one place a given binary
  looks, not a moving target.
- **Per-launch config directories need cleanup.** They live under the
  same `<items-file>.logs/` convention the item's kept output already
  uses (a sibling `.config/<id>/` directory), so they are found and
  removed the way a `work remove` already removes an item's kept output —
  tasks.md covers wiring that in, not a new cleanup mechanism.
- **`docker stop --time 0` skips Docker's own grace entirely.** This is
  deliberate (see above), but means a harness that needs more than the
  orchestrator's own `stopGrace` to flush anything on `SIGTERM` gets less
  time under docker than it might expect from `docker stop`'s normal
  default. No different in kind from the bare backend's own
  `stopGrace`-bounded `SIGTERM`-then-`SIGKILL`, which already applies the
  same bound.
- **The wrapper adds a shell script to the process tree whenever
  startup/shutdown are used**, and a hard `Kill` (the grace already run
  out) still ends everything underneath it at once — shutdown does not
  get to run to completion against a `SIGKILL`, only against the polite
  signal. This matches what "hard end" already means for the bare
  backend's own process group; it is a real limit on the "always runs"
  guarantee, not a gap specific to this change.
- **Exit code 97 is a sentinel, not a reserved range.** A harness whose
  own one-shot form happens to exit 97 for an unrelated reason cannot be
  told apart from a startup failure — mitigated by the sentinel only ever
  being read where the wrapper actually ran (i.e. only when
  startup/shutdown are named at all), so a run with no harness.yaml, or
  one naming neither script, is completely unaffected.
