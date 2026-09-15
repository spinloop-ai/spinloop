# spinloop documentation

`spinloop` points your coding agent at any model — local or hosted — with one
command. Tell it the provider you want and it configures the agent for you,
merging the settings into the config you already have instead of clobbering it.

New here? Start with **[Getting started](getting-started.md)** — install to
launched agent in a couple of minutes.

## The ideas

Four words carry the whole tool:

- **Harness** — the coding agent being configured. opencode is the default;
  Pi and lucinate are also supported. Chosen at runtime, so the same selection
  works for any of them. See [`spinloop harness`](commands/harness.md).
- **Provider** — what `spinloop` can configure, from a built-in
  catalogue: OpenRouter, AWS Bedrock, Ollama, llama.cpp, vLLM, oMLX and MTPLX
  (both Apple Silicon), or any OpenAI-compatible endpoint. See
  [`spinloop provider list`](commands/provider.md).
- **Spinloop file** — a small, declarative file (like a `Dockerfile`, but for
  your agent's model) that captures one selection so you can commit it and
  apply it anywhere — local or fetched straight from a URL. See
  [The `Spinloop` file](spinloop-file.md).
- **Alias** — a short name you register for a Spinloop file or URL, usable
  wherever a path goes. See [`spinloop alias`](commands/alias.md).

## Guides

- [Getting started](getting-started.md) — the end-to-end flow
- [The `Spinloop` file](spinloop-file.md) — syntax and examples
- [Running on a cloud GPU](commands/remote.md) — the same Spinloop, on a
  machine that stops when you do
- [The HTTP control API](http-api.md) — driving a supervised engine over
  HTTP, and the [OpenAPI contract](openapi.yaml) for writing a client
- [Running a fleet](commands/fleet.md) — one spinloop watching every machine you
  run, with a [containerised fleet](../examples/fleet-docker/) and a
  [containerised gateway](../examples/gateway-docker/) you can bring up on a
  laptop
- [Working a backlog against the fleet](work-items.md) — a file of work items,
  worked by one-shot agents at the pace the fleet allows
- [Environment variables](env-vars.md) — every variable spinloop reads
- [Runnable examples](../examples/) — ready-to-apply Spinloops with walkthroughs
- [Deploying your own cloud GPU endpoint](../remote/) — the AWS project behind
  `spinloop remote`
- [Development](development.md) — building and testing spinloop, and where to
  add a provider or another harness
- [Implementation notes](internals.md) — maintainer-facing gotchas and
  adapter schema references (behavior itself is specified in
  `openspec/specs/`)

## Commands

| Command | What it does |
| ------- | ------------ |
| [`spinloop alias`](commands/alias.md) | Name a `Spinloop` so the name works anywhere a path does |
| [`spinloop unalias`](commands/unalias.md) | Drop a registered name |
| [`spinloop serve`](commands/serve.md) | Run the inference server for the model a `Spinloop` names |
| [`spinloop up`](commands/up.md) | Start the engine this directory holds: the fleet, or the `Spinloop`'s server |
| [`spinloop code`](commands/code.md) | Launch the active harness — a one-word shortcut for `spinloop harness open` |
| [`spinloop daemon`](commands/serve.md#the-control-api---api-and-spinloop-daemon) | Supervise an engine over the [control API](http-api.md) |
| [`spinloop status`](commands/status.md) | What every engine you run is doing, one row each — a fleet, or one environment |
| [`spinloop dashboard`](commands/dashboard.md) | The live tiled view of the same, with the keys to drive it |
| [`spinloop fleet`](commands/fleet.md) | Drive the engines on every machine you run: start, stop, deploy, route |
| [`spinloop gateway`](commands/gateway.md) | Serve the fleet under one OpenAI-compatible endpoint |
| [`spinloop orchestrator`](commands/orchestrator.md) | Work a backlog of items against the fleet, at the fleet's declared pace |
| [`spinloop work`](commands/work.md) | Work the work items file from the shell: add, list, abort, remove |
| [`spinloop remote`](commands/remote.md) | Run the model on a cloud GPU that stops when you do |
| [`spinloop hf`](commands/hf.md) | Write a `Spinloop` for a Hugging Face model, from its page reference |
| [`spinloop harness`](commands/harness.md) | Configure the agent (add, remove, apply, unapply, show, export), launch it (open), and set the default (config) |
| [`spinloop provider`](commands/provider.md) | Work with the provider catalogue (list, init) |
| [`spinloop completion`](commands/completion.md) | Tab completion for your shell |

`spinloop version` prints the version, and `spinloop help` the usage summary.

## Environment variables

The ones you will meet first — **[env-vars.md](env-vars.md) is the full list**,
including the `SPINLOOP_REMOTE_*` overrides:

| Variable | Effect |
| -------- | ------ |
| `SPINLOOP_HARNESS` | Selects the harness (a `--harness`/`-H` flag beats it) |
| `SPINLOOP_ALIAS` | A registered [alias](commands/alias.md) to use when a command names no Spinloop (an argument beats it; it beats `./Spinloop`) |
| `SPINLOOP_CONFIG_DIR` | spinloop's own config directory, used verbatim — set it where there is no usable `$HOME` |
| `SPINLOOP_PROVIDERS` | Path to a custom provider catalogue (`--providers` beats it) |
| `SPINLOOP_BASE_URL` | Overrides any provider's API base URL (`--base-url`/`-u` beats it) |
| `SPINLOOP_API_TOKEN` | Bearer token for the daemon [control API](http-api.md) |
| `SPINLOOP_LOG_LEVEL` | How much `spinloop daemon`/`spinloop serve` record — `debug`, `info` (default), `warn`, `error` (`--log-level` beats it) |
| *(named by `tokenEnv`)* | A [fleet](commands/fleet.md) node's bearer token — `fleet.yaml` names the variable, never the value |
| `DEEPSEEK_API_KEY`, `OPENAI_API_KEY`, … | Provider API keys — `spinloop provider list` shows which each provider reads |
| `OLLAMA_BASE_URL`, `LLAMACPP_BASE_URL`, `OMLX_BASE_URL`, `VLLM_BASE_URL`, `MTPLX_BASE_URL`, `OPENAI_BASE_URL` | Per-provider endpoint overrides |
| `AWS_REGION` | Region for AWS Bedrock |

Keys are looked up in a `.env` file **beside the `Spinloop` being applied** first
(or in the current directory, for a command that takes no Spinloop), then your
shell environment — so a project keeps its own key next to the file that needs
it, the same way `PRESET` travels with a Spinloop. They are **never written into the agent's config** — spinloop writes
a reference the agent resolves when it runs, and `spinloop harness open` passes
the keys it can resolve to the agent it launches. If you start the agent yourself,
set the variable in your own environment. Local providers on localhost (Ollama,
llama.cpp) need no key; Bedrock uses your AWS credentials. oMLX and MTPLX need
one only if you enabled their API-key auth — set `OPENAI_API_KEY` before
applying if you did.
