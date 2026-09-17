## 1. Extract the shared plan, no behavior change

- [x] 1.1 Extract `resolvePlan` from `Dispatcher.Launch`: the directory
  check/create, the one-shot form lookup, the catalogue provider, and the
  `spinloop.Selection` — verify with `go test ./internal/orchestrator/...`
  passing unchanged (existing `Launch` tests still hold against the
  refactor alone)
- [x] 1.2 Introduce the `Launcher` interface (`Launch(Item, Node, string)
  (Child, error)`) and rename today's `Launch` body into a `bareLauncher`
  that calls `resolvePlan` then does exactly what `Launch` does today —
  verify with the existing dispatch/worklist test suite passing with no
  changes to its assertions
- [x] 1.3 Change `WorkList`'s dispatcher field to hold a `Launcher`, `NewWorkList` and `NewDispatcher`'s
  callers updated — verify with `go build ./...` and the orchestrator
  suite passing

## 2. Render a provider config without touching the host

- [x] 2.1 Add `RenderProviderConfig` to `harness.Harness`, implemented for
  opencode and Pi by factoring their `Apply`'s "no existing file" branch
  out into it, `Apply` calling the same function for that case — verify
  with a test asserting `Apply` on an empty host config and
  `RenderProviderConfig`'s bytes parse to the identical provider block
- [x] 2.2 Lucinate's harness is left without the method (it has no
  one-shot form, so the docker backend never dispatches it) — verify
  `internal/harness` still builds without a lucinate implementation, and
  document why in a comment beside the interface method

## 3. `harness.yaml` and the wrapper

- [x] 3.1 Add a `harness.yaml` loader: `--harness-config`, else
  `harness.yaml` beside the items file, else nothing, read once at
  startup — verify with a test for each of the three cases, and one for a
  present-but-unparsable file failing the command before it works an item
- [x] 3.2 Fold `harness.yaml`'s `env` into `resolvePlan`'s environment; an
  entry naming the resolved token's variable fails the command before any
  item is worked, naming the variable — verify with a test for the merge
  and one for the refusal
- [x] 3.3 Add the wrapper: a generated POSIX shell script (design.md's
  "The wrapper" decision) run in place of the harness directly whenever
  `harness.yaml` names a `startup` or `shutdown` script; where neither is
  named, launch the harness directly exactly as today — verify with a
  unit test rendering the script for startup-only, shutdown-only, both,
  and neither, and shell-checking the rendered output
- [x] 3.4 Wire the wrapper into the bare backend: `startChild` runs the
  wrapper (`/bin/sh -c <script> -- <harness bin> <harness args>`) instead
  of the harness directly when a wrapper applies, `STARTUP`/`SHUTDOWN`
  and the merged env all reaching it — verify with a test asserting a
  startup script's and a shutdown script's output both land in the log,
  in order, around the harness's own
- [x] 3.5 A wrapper exit of 97 is read as "startup failed", not the
  harness's own outcome: the item is failed naming the startup script,
  distinctly from a harness failure — verify with a test asserting the
  item's recorded failure names the script, not the harness
- [x] 3.6 An abort of a wrapped item: `Stop`/`Kill` act on the wrapper,
  which forwards the signal to the harness child and still runs shutdown
  before it exits — verify with a test using a harness stand-in that
  ignores the first signal, asserting shutdown's output is in the log
  before the abort is answered

## 4. The docker launcher

- [x] 4.1 Add `dockerLauncher` implementing `Launcher`: calls
  `resolvePlan`, then `RenderProviderConfig`, writes the bytes to a fresh
  per-launch directory under `<items-file>.logs/.config/<item-id>/` —
  verify with a test asserting the file's content and that a second
  launch for a different item gets its own directory
- [x] 4.2 Build the `docker run -d` invocation: `--network host`, the
  workspace mount (`item.Dir` → `/workspace`, the harness's working
  directory), the config mount (the per-launch directory →
  `/home/agent/<suffix>`, from the small suffix table design.md
  describes), the token and `harness.yaml`'s `env` as `-e`, the wrapper
  (§3.3) as the container's command where one applies, else the harness
  directly, the image reference — verify with a test against a fake
  `docker` seam (the same `d.start`-style test hook pattern `startChild`
  already uses) asserting the exact argv
- [x] 4.3 Implement `dockerChild`: `Wait` as `docker wait`, `Stop` as
  `docker stop --time 0`, `Kill` as `docker kill` — verify with a test
  against the fake `docker` seam asserting each method's argv and that
  `Wait`'s exit code becomes the returned error the way `procChild`'s
  does
- [x] 4.4 A docker failure (the run command's non-zero exit, or the
  binary missing) surfaces as an `error` from `Launch`, naming the item
  and the docker command's own message — verify with a test asserting the
  item is recorded failed and the rest of the backlog still runs, the way
  `TestPrune_...`-style tests already assert for a bare launch failure

## 5. The official agent image

- [x] 5.1 Add a `Dockerfile` (e.g. `images/agent/Dockerfile`) building an
  image with opencode, Pi and `gh` installed, a fixed `agent` user,
  `HOME=/home/agent` — verify by building it locally and running `docker
  run --rm <image> opencode --version`, `... pi --version` and `... gh
  --version` all succeeding
- [x] 5.2 Confirm the image carries no provider configuration out of the
  box — verify by running the image with nothing mounted and checking
  `~/.config/opencode` and `~/.pi` are absent or empty
- [x] 5.3 Wire a CI job that builds and publishes the image on a tag,
  named to match `main.version` the way the `spinloop` binary's own
  release does — verify the workflow's dry run (or a local build)
  produces the expected tag string

## 6. The `--dispatch` flag and startup checks

- [x] 6.1 Add `--dispatch` to `spinloop orchestrator` (`bare` default,
  `docker` the alternative), constructing the matching `Launcher` — an
  unrecognised value fails the command before it works an item, naming
  the flag, the value, and the accepted set — verify with a test
  asserting the refusal's message and that no item is worked
- [x] 6.2 Where `--dispatch docker` is given, run `docker version` once
  at startup; a daemon that does not answer fails the command the way an
  unreachable gateway already does, naming the fix — verify with a test
  against the fake `docker` seam
- [x] 6.3 Add `--dispatch-image`, defaulting to
  `ghcr.io/spinloop-ai/agent:<spinloop's version>`, only meaningful under
  `--dispatch docker` — verify with a test asserting the default's
  version matches `main.version` and that the flag overrides it
- [x] 6.4 Add `--harness-config`, naming the file §3.1's loader reads —
  verify with a test asserting the flag's file is read in preference to
  a `harness.yaml` beside the items file

## 7. Cleanup

- [x] 7.1 `work remove` and the state's own end-of-run cleanup take the
  per-launch config directory out with the item's kept output — verify
  with a test asserting the directory is gone after a remove, the way the
  log file already is

## 8. Documentation and the spec

- [x] 8.1 Document `--dispatch`, `--dispatch-image`, `--harness-config`,
  the docker backend's mounts and network choice, and `harness.yaml`'s
  `env`/`startup`/`shutdown` in `docs/commands/orchestrator.md` — verify
  by reading it against the `agent-dispatch` spec's scenarios
- [x] 8.2 A short README under `images/agent/` describing what the image
  carries and how its tag is chosen — verify by reading it against
  design.md's "Image tag follows the spinloop version" decision

## 9. Tests and the suite

- [x] 9.1 Run the full suite with the race detector — verify with `go
  test ./... -race -cover` passing, no regressions, `internal/orchestrator`
  and `internal/harness` coverage holding at or above 80%
