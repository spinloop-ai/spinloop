<p align="center">
  <img src="assets/logo.png" alt="spinloop" width="520">
</p>

<p align="center">
  Point your coding agent at any model — local or hosted — with one command.
</p>

<p align="center">
  <em>// no hand-editing JSON, no model ids to memorise, no clobbering the config you already have.</em>
</p>

---

```sh
# point your coding agent at a model — here, a local Qwen3.6 on Ollama
spinloop add -p ollama -m qwen3.6

# launch that agent, now running the model you picked
spinloop harness

# prefer it declarative? commit an ./Spinloop file and apply it
spinloop apply

# running that model locally too? the same file launches the server
spinloop serve
```

That's the whole tool. Your agent is configured and pointed at the model; the rest
of your config never moved.

## Supported providers

Every provider below is built in — name it with `-p` and `spinloop` fills in the
base URL, package and key variable for you. Run `spinloop list` to see them with
their models.

| Provider | `-p` name | What it is |
| --- | --- | --- |
| OpenRouter | `openrouter` | Hosted model aggregator — hundreds of models behind one key |
| AWS Bedrock | `amazon-bedrock` | Claude and other models, authenticated with your AWS credentials |
| Google Vertex AI (Gemini) | `google-vertex` | Gemini models on GCP, via your Google credentials |
| Google Vertex AI (Claude) | `google-vertex-anthropic` | Anthropic Claude on GCP, via your Google credentials |
| Ollama | `ollama` | Local Ollama server |
| llama.cpp | `llamacpp` | Local (or remote) llama-server |
| oMLX | `omlx` | Local oMLX server on Apple Silicon |
| vLLM | `vllm` | Local or self-hosted vLLM server |
| OpenAI-compatible | `openai-compatible` | Any endpoint that speaks the OpenAI API — set the base URL and key |

Adding one that isn't here is a data change, not code — see
[Adding providers and models](#adding-providers-and-models).

---

Your coding agent is only as good as the model behind it, and the model you want
changes by the day — a frontier model on OpenRouter for the hard stuff, a local
Qwen on llama.cpp when you're offline or cost-conscious, Claude on Bedrock for
work. Switching between them should take a second. It usually doesn't.

Every agent keeps its config somewhere different, in a shape of its own. Pointing
one at a new provider means opening that file by hand and getting four things
exactly right: the base URL, the model id, the package it loads, and the name of
the environment variable holding your key. One stray brace and the agent won't
start. **Local models are the worst of it** — each runtime has its own ports,
model refs and quirks, and none of it is written down where you need it.

`spinloop` is the supply line for your coding agent. Tell it the provider you want and
it configures the agent for you:

- **One command, any model.** Pick from a built-in catalogue — OpenRouter,
  Bedrock, Ollama, llama.cpp, vLLM, oMLX, or any OpenAI-compatible endpoint. No
  URLs to look up, and `spinloop list --models` fetches the model ids straight from
  the provider.
- **Your config survives.** Settings are merged *into* what you already have.
  Other providers, your theme, even your comments stay exactly where you left them.
- **Keys stay where they belong.** Secrets are read from a local `.env` and never
  hard-coded somewhere they'll leak — written `0600`, or kept as an env reference.
- **Local models, sorted.** The same file that points your agent at a local model
  can launch the server for it. One source of truth, two jobs.

Works with [opencode](https://opencode.ai),
[Pi](https://github.com/earendil-works/pi) and
[lucinate](https://github.com/lucinate-ai/lucinate) today — pick the one you use
per command, or set a default. The same selection works for any of them.

## Install

With [Homebrew](https://brew.sh):

```sh
brew install spinloop-ai/tap/spinloop
```

To upgrade later, run `brew upgrade spinloop`.

### From source

```sh
go build -o spinloop ./cmd/spinloop
```

Drop the resulting `spinloop` binary anywhere on your `PATH`.

## Quickstart

See what's in the catalogue:

```sh
spinloop list
```

Need a model id? Ask the provider itself — no memorising, no guessing:

```sh
spinloop list --models openrouter    # the models it currently serves, live
```

Add a provider and a model:

```sh
# OpenRouter needs a key — put it in .env first:
echo 'DEEPSEEK_API_KEY=sk-or-v1-...' > .env

spinloop add --provider openrouter --model deepseek/deepseek-v4-flash
```

Then just run `opencode`. That's it — your agent is pointed at the new model, and
the rest of your config is untouched.

### More examples

```sh
# A local Ollama model (no key required)
spinloop add -p ollama -m llama3.2

# Claude on AWS Bedrock (uses your AWS credentials)
spinloop add -p amazon-bedrock -m anthropic.claude-3-5-sonnet

# Any OpenAI-compatible endpoint, base URL via flag
OPENAI_API_KEY=sk-... \
  spinloop add -p openai-compatible -m my-model --base-url https://my-endpoint/v1

# Pin a specific default model
spinloop add -p openrouter -m deepseek/deepseek-v4-pro

# Set the context window — human suffixes or an absolute count, both fine
spinloop add -p llamacpp -m my-model -c 128k
spinloop add -p llamacpp -m my-model --context 200000

# Cap the max output tokens too (defaults to a quarter of the context)
spinloop add -p llamacpp -m my-model -c 128k -o 32k

# Take a provider back out
spinloop remove -p ollama

# Or just drop one model
spinloop remove -p openrouter -m deepseek/deepseek-v4-flash
```

On opencode, `add` sets the chosen model as the default and `remove` clears it
if it pointed at something you removed. Pi has no default-model setting, so
`add` just registers the provider and tells you which model to pick with `/model`.
On lucinate, `add` writes an OpenAI-compatible connection and points lucinate's
startup default at it, so it opens straight onto the model you chose.

`--context`/`-c` records each added model's context window. Parsing is
forgiving: `128k`, `1m`, `1.5m`, `200000`, `128,000`, even `128 K tokens` all
land where you'd expect (`k`/`m`/`g` are decimal — `128k` is 128,000 tokens).

`--output`/`-o` caps the max output tokens, in the same format. opencode needs
one whenever a context is set, so when you leave it off `spinloop` fills in a
quarter of the context for you. It can't exceed the context window.

## Usage

```sh
spinloop list   [--models [<provider>]]    # the catalogue; --models fetches live model ids
spinloop show   [--harness <name>]         # show what the harness has configured
spinloop add    --provider <name> [--model <id>] [--alias <name>] [--context <size>] [--output <size>] [--base-url <url>]
spinloop remove --provider <name> [--model <id>] [--alias <name>]
spinloop apply  [path] [--output <size>]   # apply a Spinloop file or directory (default ./Spinloop)
spinloop unapply [path]                    # remove what a Spinloop file selects
spinloop alias  [path] [-n <name>] [-l]    # name a Spinloop; -l lists them
spinloop unalias <name>                    # drop a registered name
spinloop serve  [path] [--dry-run] [-a]    # run the PROVIDER's inference server, from the PRESET
                                         #   (-a/--api serves the control API beside it)
spinloop daemon [--api-addr <addr>] [--loopback] # supervise an engine via the control API — reads
                                         #   no Spinloop, starts nothing until asked over the API
spinloop fleet <status|metrics|logs|dashboard|route|start|stop>
                                          # observe and drive the engines in
                                          #   fleet.yaml (dashboard is the
                                          #   interactive tiled view)
spinloop export [--provider <name>]        # print the current config as a Spinloop
spinloop init-providers [path]             # write the built-in catalogue out to edit
spinloop harness [<spinloop>] [-H <name>] [--spinloop[=<path>]] [args...]
                                         # launch the harness (a leading Spinloop or alias is
                                         #   applied first; --get shows it; --set stores it)
spinloop completion <shell>                # tab completion (bash, zsh, powershell)
spinloop remote <bootstrap|start|pause|stop|status|metrics|logs|deploy|env|ls|keep> [path]
                                         # control the remote GPU inference instance
                                         #   (bootstrap does the once-per-account setup;
                                         #    deploy sets what it serves, from the Spinloop;
                                         #    pause stops it while keeping it re-wakeable;
                                         #    keep holds it against the idle sweep;
                                         #    logs reads the shipped logs, alive or not;
                                         #    env prints the running endpoint's env vars)
```

Short flags: `-p` (provider), `-m` (model), `-a` (alias), `-c` (context), `-o` (output), `-u` (base-url), `-H` (harness), `-O` (spinloop), and under `alias`: `-n` (name), `-l` (list), `-F` (force).

Anywhere a `[path]` appears above you can put a name registered with
[`spinloop alias`](#aliases) instead, an `http(s)` URL, fetched instead of read
from disk — or leave it out and let `SPINLOOP_ALIAS` name one.

## Documentation

The [`docs/`](docs/) directory is the user manual:

- [Getting started](docs/getting-started.md) — install to launched agent, end
  to end
- [The `Spinloop` file](docs/spinloop-file.md) — full syntax and examples
- [Command reference](docs/README.md#commands) — a page per command, under
  [`docs/commands/`](docs/commands/)

## Harnesses

A **harness** is the coding agent being configured. opencode is the default; Pi
and lucinate are also supported. The harness is chosen at runtime — never baked
into a `Spinloop` file — so the same selection works for any of them.

```sh
spinloop add -p ollama -m llama3.2 --harness pi   # this command only
spinloop harness --set pi    # make Pi the default for future commands
spinloop harness             # launch the active harness (forwards trailing args)
spinloop harness -O          # apply ./Spinloop, then launch the harness
spinloop show                # what the active harness has configured
```

Precedence: `--harness`/`-H` flag, then `SPINLOOP_HARNESS`, then your stored
default, then opencode. Not every provider maps to every harness — `spinloop list`
shows which harnesses each one supports (AWS Bedrock, for instance, is
opencode-only; lucinate takes the OpenAI-compatible providers). The full story —
launching, configuring on the way in, inspecting
any harness — is in [`docs/commands/harness.md`](docs/commands/harness.md) and
[`docs/commands/show.md`](docs/commands/show.md).

## Spinloop files

Prefer to keep a provider selection in a file — like a `Dockerfile`, but for
your coding agent? Drop a `Spinloop` in your project:

```dockerfile
# Spinloop
PROVIDER openrouter
MODEL    deepseek/deepseek-v4-pro   # the provider-native model ref
ALIAS    deepseek                   # optional; friendly name for the model
CONTEXT  128k                       # optional; context window
OUTPUT   32k                        # optional; max output tokens
BASEURL  https://gateway/v1         # optional; API base URL override
```

```sh
spinloop apply              # reads ./Spinloop and applies it
spinloop apply path/to/Spinloop
spinloop apply path/to/dir  # or a directory that holds a Spinloop
spinloop apply https://example.com/team/Spinloop   # or a URL, fetched instead of read
spinloop harness -O         # apply ./Spinloop, then launch the agent running it
spinloop export > Spinloop    # capture your current setup as a Spinloop
```

A `Spinloop` describes one provider selection and applies exactly like the
equivalent `add`. Full syntax is in [`docs/spinloop-file.md`](docs/spinloop-file.md),
and ready-to-use examples live under [`examples/`](examples/), including
[fetching one from a URL](examples/remote-spinloop/).

## Aliases

Keeping a directory per model soon means typing a path per command. Name one
once with `spinloop alias` and the name works wherever a path does:

```sh
$ spinloop alias
Added alias "qwen3.6-27b" for /home/me/models/qwen3.6/Spinloop …

$ spinloop apply   qwen3.6-27b      # from anywhere, no path needed
$ spinloop serve   qwen3.6-27b
$ spinloop harness qwen3.6-27b -- --some-agent-arg
```

The path can be a URL too — hand out a short name for a published `Spinloop`
instead of a link:

```sh
spinloop alias -n team-default https://example.com/team/Spinloop
spinloop apply team-default
```

Set `SPINLOOP_ALIAS` and the name is implied for a whole shell:

```sh
export SPINLOOP_ALIAS=qwen3.6-27b
spinloop apply              # the same as `spinloop apply qwen3.6-27b`
spinloop serve
```

An argument you type still wins, and the variable beats `./Spinloop` — it decides
*which* Spinloop is the default, never *whether* one is applied, so a bare
`spinloop harness` still launches unconfigured.

The name defaults to the `Spinloop`'s own `ALIAS` (`--name`/`-n` picks another),
a path on disk always beats a registered name — so adding an alias can never
change what an already-working command does — and the registry lives in
`spinloop`'s own config, never in a `Spinloop`, so your files stay portable and
committable. Listing, re-pointing, and `unalias` are covered in
[`docs/commands/alias.md`](docs/commands/alias.md).

### Tab completion

```sh
source <(spinloop completion bash)   # add to ~/.bashrc
source <(spinloop completion zsh)    # or ~/.zshrc (needs compinit)
spinloop completion powershell | Out-String | Invoke-Expression   # or $PROFILE
```

TAB then completes commands, flags, providers, harnesses, and your registered
aliases — details in
[`docs/commands/completion.md`](docs/commands/completion.md). Homebrew installs
the bash and zsh completions for you.

## Serving a local model

Running a model locally? `spinloop serve` reads a `Spinloop` and launches the
inference server its `PROVIDER` names — `llamacpp` runs `llama-server`, `omlx`
runs [oMLX](https://omlx.ai) on Apple Silicon — so the same file that points
opencode at a model can start it too. The simple case needs no preset:

```dockerfile
# Spinloop
PROVIDER llamacpp
MODEL    unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL   # HF repo, or a .gguf path
ALIAS    qwen3.6                                    # llama-server --alias
CONTEXT  32768                                      # llama-server --ctx-size
# PARALLEL 2                                        # optional; concurrent slots
```

```sh
spinloop serve              # builds a llama-server command and runs it
spinloop serve --dry-run    # just print the command — no server
```

For flags a `Spinloop` doesn't model (`-ngl`, `--jinja`, KV-cache types, draft
models), point at a llama.cpp preset `.ini` with `PRESET` and `serve` flattens
the chosen section into the command instead — with anything the `Spinloop` states
(like `CONTEXT`) overriding the preset. It's the missing piece presets don't
cover: launching a *single* model. `CONTEXT` always means the context per
request; add `PARALLEL` to run more than one slot and `spinloop` works out each
engine's own accounting (llama.cpp's `--ctx-size` gets scaled, vLLM's and
oMLX's don't) — see
[Parallelism](docs/commands/serve.md#parallelism) for the full mapping.
Details in [`docs/commands/serve.md`](docs/commands/serve.md).

### The daemon

`spinloop daemon` runs a long-lived agent that supervises one engine and serves
a small control API: status, start, stop, metrics — token counters scraped
from the engine plus GPU/CPU/RAM readings from the host — and a deploy-config
push. It's a worker: it reads no Spinloop, no preset, and no `fleet.yaml`, and
takes no Spinloop path of its own — what it runs is decided entirely by
whoever asks it, over the API. It starts *nothing* on boot: the engine runs
only when a start request asks, and the request can carry the deploy config
(runner, model, flags) to run — or fall back to a previously pushed one. With
neither, a start says so. Stopping the engine leaves the daemon answering.

It also keeps an eye on whether the engine is actually doing anything: it
reads the engine's counters every 15 seconds and reports `lastActiveAt` and
`idleSeconds` on both `/v1/status` and `/v1/metrics`, from one record, so the
two cannot disagree. Ask the daemon how busy it is rather than working it out
from raw counters yourself.

```sh
SPINLOOP_API_TOKEN=…  spinloop daemon           # control API on :4242
spinloop daemon --loopback                    # loopback-only (127.0.0.1:4242), needs no token
spinloop daemon --api-token-file /run/secrets/spinloop-token   # from a service manager
spinloop daemon --log-level warn              # quiet on a node a fleet polls
```

The API is bearer-token authenticated — `SPINLOOP_API_TOKEN`, `--api-token`, or
`--api-token-file` (giving two at once is an error, since the daemon reads no
Spinloop and so has no adjacent `.env` to fall back to); a non-loopback listen
without a token refuses to start — which is exactly what `--loopback` is for.
`spinloop serve -a/--api` exposes the same API beside an ordinary foreground
serve, and — because that command *does* read a Spinloop — takes its token
from the environment as usual, `.env` included.

Every request is summarised on stderr — method, path, status, duration, size,
caller — alongside the engine's starts, stops and crashes. Never the token and
never a body. `--log-level warn` keeps a polled node quiet without hiding the
rejections; see [what gets logged](docs/commands/serve.md#what-gets-logged).

### The fleet

With a daemon on each machine, `spinloop fleet` observes them all. A
`fleet.yaml` names the nodes — and holds no secrets, referencing each node's
token by environment-variable name:

```yaml
nodes:
  - name: studio
    host: studio.local
    tokenEnv: STUDIO_TOKEN
  - name: gpu-box
    host: 198.51.100.7    # e.g. a tailscale address
    tokenEnv: GPU_BOX_TOKEN
```

```sh
spinloop fleet status          # one row per node: state and what it serves
spinloop fleet dashboard       # the interactive tiled view — watch it, drive it
spinloop fleet start gpu-box   # start one node's engine
```

`dashboard` is the fleet you actually look at: one tile per node, repainted
in place, each drawing what `fleet metrics` prints — start a node with `s`,
stop one with `x`, and a waking cloud machine reports its own progress on its
tile; `a` ends the wait on one in flight (the wake goes on in the cloud).
Press `<enter>` on a tile for a full-screen view of that node — metrics, its
engine log tailed live, and the keys that work there — `<esc>` to go back.
`fleet metrics --watch` is the same board as a stream, for pipes.

```
NODE     STATE         SERVING
studio   running       llamacpp  org/qwen  (up 1h 2m 5s)  (last active 12s ago)
gpu-box  idle          llamacpp  org/qwen
offline  unreachable   dial tcp 10.0.0.9:4242: connect: connection refused
```

A node that cannot be reached is a row, not a failure — one bad box never
blanks the view, and "last active" answers the question you actually opened the
thing for: which machine is doing nothing?

No spare machines to hand? [`examples/fleet-docker/`](examples/fleet-docker/)
brings up a three-node fleet in containers — real daemons, real auth, a fake
engine — in about a minute:

```sh
cd examples/fleet-docker && cp .env.example .env
docker compose up -d --build
set -a && . ./.env && set +a
spinloop fleet status --fleet ./fleet.yaml
```

Only one machine? A fleet of one is still worth it —
[`examples/fleet-local/`](examples/fleet-local/) runs a daemon on your own box
so `spinloop harness` starts the engine when you need it and leaves it up for the
next session, instead of you keeping a terminal open for `llama-server`.

Details in [`docs/commands/fleet.md`](docs/commands/fleet.md); a `fleet.yaml`
for machines you own in [`examples/fleet/`](examples/fleet/).

Writing a client? [`docs/openapi.yaml`](docs/openapi.yaml) is the full
contract, and it ships with every release. See
[`docs/http-api.md`](docs/http-api.md) for the endpoints in prose.

## Remote inference instance

Running a model on your own cloud GPU box? [`remote/`](remote/) deploys one.
`spinloop remote` drives its scale-to-zero lifecycle: the instance only exists
while you are using it, and stops itself after a period of idleness.

```sh
spinloop remote start     # boot the instance, wait for the model to load,
                         # then print OPENAI_BASE_URL / OPENAI_API_KEY exports
spinloop remote status    # instance state, endpoint health, and when it last
                         # did any work
spinloop remote metrics   # tokens, GPU, CPU and RAM — plus the same last-active
spinloop remote logs      # what the engine (or the boot) said, even after it's gone
spinloop remote pause     # stop now, but keep it re-wakeable
spinloop remote restart   # fresh engine, same address: stop it, then wake it
spinloop remote keep 4h   # hold it against the idle sweep for 4 hours
                         # (start --keep does the same at wake time)
spinloop remote stop      # terminate now instead of waiting for the idle timer
```

Instances ship their engine and boot output to CloudWatch, so `spinloop remote
logs` still works once the instance has terminated — including for a start that
failed before the engine came up (`--source boot`). See
[docs/commands/remote.md](docs/commands/remote.md#reading-the-logs).

Configuration lives in a `remote.json`. A project's `Spinloop` file can name
one with a `REMOTE` instruction — either a path (`REMOTE remote.json`,
resolved relative to the Spinloop, like `PRESET`, so the pair travel together)
or the name of a registered environment (`REMOTE dev-2`, whose file sits at
`remotes/dev-2/remote.json` under spinloop's config directory,
`${SPINLOOP_CONFIG_DIR:-${XDG_CONFIG_HOME:-~/.config}/spinloop}`). With no
`REMOTE`, the `default` environment is used. Either way the file takes the
`SpinloopRemoteConfig` output of the `remote/` deployment — or `spinloop remote
deploy` writes it when it registers the environment:

```json
{"start_url": "https://...lambda-url...on.aws/", "stop_url": "https://...", "region": "eu-west-1", "base_url": "http://198.51.100.7:8000/v1"}
```

`base_url` is the endpoint's own address. `remote` doesn't need it — `start`
and `status` report the address themselves — but `spinloop apply` reads it, so an
Spinloop with a `REMOTE` line can leave `BASEURL` out and still point your agent
at the endpoint. A `BASEURL` in the Spinloop takes precedence.

Every URL and the region can be overridden with the matching
[`SPINLOOP_REMOTE_*`](docs/env-vars.md) environment variable. Requests are
SigV4-signed
with your AWS credentials (env, profile or SSO — the standard chain), which
must be allowed `lambda:InvokeFunctionUrl`. A cold `start` takes a few
minutes while the instance boots and loads the model; `--timeout` (default
15m) caps the wait.

The remote commands read the `.env` beside the Spinloop before they sign, so the
AWS credentials, region and `SPINLOOP_REMOTE_*` overrides can travel with the
Spinloop. A value already set in your shell wins over the `.env`. To pin a value
in the Spinloop itself, add an `ENV` line (`ENV AWS_PROFILE=prod`) — it may repeat
and overrides both the `.env` and your shell. `ENV` applies only on your
machine; it is never sent to the deployed instance.

## Keys and endpoints

Each provider declares which environment variable holds its key (`spinloop
list` shows them). Values are looked up in your shell environment first, then a
`.env` beside the `Spinloop`, so an exported variable always wins and the `.env`
only fills a gap. Local providers like Ollama, llama.cpp and oMLX need no
key;
Bedrock authenticates through your AWS credentials.

`spinloop harness` carries that same local environment to the agent it launches:
the whole `.env` beside the active Spinloop fills gaps, and the Spinloop's `ENV` lines
override both your shell and the `.env` — the same precedence the `spinloop remote`
commands use. These variables shape only the launched agent; `spinloop` never
changes its own environment.

Base URLs default to the usual local ports. Override the endpoint for **any**
provider with `--base-url`/`-u` or the `SPINLOOP_BASE_URL` env var — handy for
proxies, gateways, or a server on a non-default host:

```sh
spinloop add -p openai-compatible -m my-model --base-url https://gateway/v1
SPINLOOP_BASE_URL=https://gateway/v1 spinloop add -p openai-compatible -m my-model
```

The flag wins over the env var, and either wins over the catalogue's defaults
and the per-provider variables (`OLLAMA_BASE_URL`, `LLAMACPP_BASE_URL`,
`OMLX_BASE_URL`, `VLLM_BASE_URL`, `OPENAI_BASE_URL`).

## Guides

Provider- and model-specific walkthroughs live in [`examples/`](examples/), each
with a ready-to-apply `Spinloop`:

- [Qwen3.8-27B on llama.cpp — local or deployed to AWS](examples/llamacpp/qwen3.8-27b/README.md)
- [Qwen3.6-27B on llama.cpp](examples/llamacpp/qwen3.6-27b/README.md)
- [Qwen3.6-35B-A3B on llama.cpp](examples/llamacpp/qwen3.6-35b-a3b/README.md)
- [Gemma-4-12B-IT on llama.cpp](examples/llamacpp/gemma4/README.md)
- [Qwen3.6-35B-A3B on oMLX (Apple Silicon)](examples/omlx/qwen3.6/README.md)
- [Gemma-4-E2B on oMLX (Apple Silicon)](examples/omlx/gemma-4-e2b/README.md)
- [Fetching a Spinloop from a URL](examples/remote-spinloop/README.md)

## Adding providers and models

Everything `spinloop` knows lives in `internal/catalog/providers.yaml`. Add a
provider there and rebuild — no Go required. The
file is commented with the schema.

Don't want to rebuild? Write the catalogue out with
[`spinloop init-providers`](docs/commands/init-providers.md), edit it, and point
`spinloop` at it at runtime — the flag wins, then the env var, then the built-in
default:

```sh
spinloop init-providers                 # writes ./providers.yaml
spinloop list --providers providers.yaml
SPINLOOP_PROVIDERS=providers.yaml spinloop list
```

## Development

`spinloop` is a Go CLI with no runtime dependencies. The domain logic is split
into `internal/` packages so each concern is isolated and independently testable;
[`AGENTS.md`](AGENTS.md) is the map of how it all fits together.

```sh
go build -o spinloop ./cmd/spinloop   # build the binary
go test ./...                     # run the suite
go test ./... -cover              # with coverage (kept >= 80%)
go vet ./...                      # vet
gofmt -w ./...                    # format
```

## Contributing

Issues and pull requests are welcome. A few things that make a change easy to
merge:

- Adding a provider or model? It's a data change in
  `internal/catalog/providers.yaml`, not Go — see
  [Adding providers and models](#adding-providers-and-models).
- Adding another harness? Start at the `Harness` interface in
  `internal/harness`; [`AGENTS.md`](AGENTS.md) walks through the contract.
- Keep the suite green and formatted (`go test ./...`, `gofmt -w ./...`) before
  opening a PR.

The `.env` file and the built binary are git-ignored.

## License

[MIT](LICENSE).
