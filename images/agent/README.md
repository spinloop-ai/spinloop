# The spinloop agent image

What `spinloop orchestrator --dispatch docker` runs an item's agent in.
Built from [`Dockerfile`](Dockerfile).

## What it carries

- [opencode](https://github.com/anomalyco/opencode) and
  [Pi](https://github.com/earendil-works/pi), the two harnesses the
  orchestrator can dispatch as a one-shot agent (lucinate has no
  single-task form, so it is never dispatched under any backend, and is
  not in this image)
- `ripgrep`, opencode's own runtime dependency
- [`gh`](https://cli.github.com/), the GitHub CLI — an item's instructions
  routinely want it: opening a pull request, reading an issue, checking a
  run's status
- A fixed, unprivileged user, `agent`, with `HOME=/home/agent`

It carries none of any operator's own harness configuration: a container
started from it with nothing else mounted in has no provider configured
for either harness. The docker dispatch backend mounts an item's own
`workspace/` and `config/` subdirectories in at `/item/workspace` and
`/item/config`, redirecting each harness's own config resolution there
with its own environment variable (`XDG_CONFIG_HOME` for opencode,
`PI_CODING_AGENT_DIR` for Pi) rather than into this image's fixed
`/home/agent` — see
[`internal/orchestrator/dispatch_docker.go`](../../internal/orchestrator/dispatch_docker.go)
and the `agent-dispatch` spec (`openspec/specs/agent-dispatch/spec.md`
once this change is archived).

## Tag

Published as `ghcr.io/spinloop-ai/agent:<version>`, one tag per spinloop
release — the same version `spinloop version` reports — so a given
spinloop binary's default `--dispatch-image` always names the image built
alongside it. `--dispatch-image` overrides the default, for testing an
unreleased image or pinning one.

## Building it locally

```sh
docker build -t spinloop-agent:local images/agent
docker run --rm spinloop-agent:local opencode --version
docker run --rm spinloop-agent:local pi --version
docker run --rm spinloop-agent:local gh --version
```

`OPENCODE_VERSION` and `PI_VERSION` build args pin the harness versions
the image installs; the CI workflow that publishes a release's image
passes the versions current at that time.
