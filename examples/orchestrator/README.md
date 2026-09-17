# A backlog you can actually run

[`spinloop orchestrator`](../../docs/commands/orchestrator.md) working a
one-item backlog against a real (if fake-engined) gateway, with a
[`harness.yaml`](harness.yaml) shaping the launch — so you can see what an
item's environment and lifecycle hooks actually do before pointing this at
real work.

This example holds no gateway of its own: it points at
[`examples/gateway-docker`](../gateway-docker/), which is real enough for
this — a real `spinloop gateway` in front of two real daemons, answering
OpenAI-shaped chat completions from a fake engine. An opencode or Pi launch
through it gets back a real (canned) reply, not a connection refused.

```sh
# once, in examples/gateway-docker/
cp .env.example .env
docker compose up -d --build
set -a && . ./.env && set +a

# from this directory — picks up ./work.yaml and ./harness.yaml, both by
# being the defaults beside the command
spinloop orchestrator --fleet ../gateway-docker/fleet.yaml -l
```

In another terminal:

```sh
spinloop work list --url http://127.0.0.1:4010
```

`describe-repo` runs once admitted, `harness.yaml`'s `startup` sets a
throwaway git identity before the agent starts, and its `shutdown` line lands
in the kept log after the agent's own output:

```sh
cat work.yaml.logs/describe-repo.log
```

## Trying `--dispatch docker`

The same backlog, the same `harness.yaml`, the agent in a container instead
of a bare process — build the image once, from the repository root:

```sh
docker build -t spinloop-agent:local -f ../../images/agent/Dockerfile ../../images/agent

spinloop orchestrator --fleet ../gateway-docker/fleet.yaml -l \
  --dispatch docker --dispatch-image spinloop-agent:local
```

`docker ps` shows the container while `describe-repo` runs. Everything else —
the work list, the log, `harness.yaml`'s hooks — is the same either way; that
sameness is the point of `--dispatch` being one flag rather than two
different things to learn.

## The files

| File | What it is |
| --- | --- |
| [`work.yaml`](work.yaml) | One item: read this directory's own README and summarise it. Change `dir` to point anywhere you actually want an agent working. |
| [`harness.yaml`](harness.yaml) | Environment variables and `startup`/`shutdown` scripts, applied to every launch under either `--dispatch` backend. Found automatically, beside `work.yaml`. |

## See also

- [`spinloop orchestrator`](../../docs/commands/orchestrator.md) — the
  command this example drives, `--dispatch` and `harness.yaml` included
- [`examples/gateway-docker/`](../gateway-docker/) — the gateway this points
  at, and what is real and fake about it
- [`images/agent/`](../../images/agent/) — the official image `--dispatch
  docker` runs
