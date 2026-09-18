## Why

The orchestrator's dispatch has exactly one path today: `Dispatcher.Launch`
runs the harness as a bare `exec.Command` on the same machine as the
orchestrator, in its own process group, with no isolation beyond that. An
item's agent reads and writes the item's directory, and whatever else its
process can reach, directly on the orchestrator's host. Operators want
stronger isolation for less-trusted work — running the agent inside a
container rather than bare on the host — and the dispatch path has to
become a choice rather than a given for that to be possible.

## What Changes

- `Dispatcher` gains a pluggable backend: the launch mechanics (how a
  harness process actually starts, and what it runs inside) move behind an
  interface, with the existing bare-process behavior as the first backend
  and a new docker backend as the second.
- `spinloop orchestrator` takes a `--dispatch` flag choosing the backend
  for every launch the run makes (`bare`, the default, or `docker`). This
  is a run-wide choice, not per-node or per-item.
- Under `docker`, an item's agent runs inside a container from an official
  spinloop image, on the container's own network — a loopback-bound
  gateway address is rewritten to `host.docker.internal` for the
  container's own rendered config, with no change to the fleet file or
  the gateway flag, and no reliance on a Docker Desktop setting that
  makes `--network host` behave the way it does on native Linux docker —
  with two bind mounts: the item's directory as the harness's working
  directory, and a
  scoped, per-launch config directory generated fresh for that one launch,
  mounted directly at the path the harness resolves its own config to
  (`$HOME`-relative, the same convention `internal/opencode`,
  `internal/pi` and `internal/lucinate` already use) — no reliance on the
  harness supporting a config-path override, and no mount of the host's
  real harness config.
- The official image is a new artifact this change publishes: opencode and
  Pi's one-shot forms, their runtimes, the `gh` CLI (an item's instructions
  routinely want it — opening a PR, reading an issue), nothing of the
  host's own configuration baked in.
- An operator's `harness.yaml` — found beside the items file by default, or
  named with `--harness-config` — configures the run's harness beyond what
  a fleet file or flag reaches: a `dispatch`, naming the backend the way
  `--dispatch` does (an explicit flag still wins), an `env` map added to
  the launch's environment, and `startup`/`shutdown` shell scripts
  bracketing it. Both backends honour it: for bare, the scripts run on
  the host in the item's
  directory; for docker, inside the container. A failing startup script
  fails the item before the harness ever runs; shutdown always runs once
  the harness has ended, whatever ended it — done, failed, or aborted —
  since it exists to tear down whatever startup set up. Both scripts'
  output joins the harness's own in the item's kept log, in order.

## Capabilities

### New Capabilities

- `agent-dispatch`: how the orchestrator runs an admitted item's harness
  process — the backend it chooses, and what each backend guarantees. The
  bare backend is today's behavior, restated as a backend rather than the
  only path; docker is the first alternative.

### Modified Capabilities

(none — `fleet-orchestrator`'s "Running an item" requirement already
states the outcome an agent's run must reach: one-shot, in the item's
directory, inference pointed at the gateway, output kept. That contract
holds under either backend by design; only the mechanics of reaching it are
new, and those belong to `agent-dispatch`.)

## Impact

- `internal/orchestrator/dispatch.go`: the `Dispatcher`/`Child` split
  becomes a launcher interface with two implementations
- `cmd/spinloop/orchestrator.go`: the `--dispatch` flag
- A new `internal/orchestrator` (or sibling) package for the docker
  backend, using the Docker Engine API or CLI to run and stop containers
- A new image build (Dockerfile, and its publishing) under, e.g.,
  `images/agent/` — exact layout decided in design.md
- `internal/orchestrator`: a `harness.yaml` loader, and the wrapper that
  bare and docker launches alike run in place of the harness directly,
  when there is a startup or shutdown script to bracket it with
- `openspec/specs/agent-dispatch/spec.md`: new capability

## Non-Goals

- Docker's stronger, VM-based sandbox product — a future backend behind
  the same interface, not built now
- Remote dispatch — running an item's agent on a machine other than the
  orchestrator's own host — also a future backend; `--dispatch` chooses
  among local backends only for now
- Per-node or per-item backend selection: one backend for the whole run
- A host-loopback service *other than the gateway* being reachable as
  plain `localhost` from inside the container: the container is on its
  own network, and only the gateway address is rewritten to reach the
  host; an item needing another host-loopback service reaches it via
  `host.docker.internal` itself
