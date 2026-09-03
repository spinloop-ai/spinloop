# The `Spinloop` file

An **Spinloop** is a small, declarative file that captures one provider
selection — which provider, and which model — so you can
apply it with a single command instead of remembering flags. Think of it like a
`Dockerfile`, but for pointing your coding agent at a model.

```dockerfile
# Spinloop — point your coding agent at one provider
PROVIDER openrouter
MODEL    deepseek/deepseek-v4-pro   # the provider-native model ref
ALIAS    deepseek                   # optional; friendly name for the model
CONTEXT  128k                       # optional; context window (per request)
OUTPUT   32k                        # optional; max output tokens
PARALLEL 2                          # optional; concurrent request slots for `serve`
BASEURL  https://gateway/v1         # optional; API base URL override
PRESET   ./preset.ini               # optional; engine preset for `spinloop serve`
```

Applying it is the same as running the equivalent
[`spinloop add`](commands/add.md), so everything you already have in your coding
agent's config is preserved.

The **harness** (opencode or Pi) is deliberately *not* part of a Spinloop — so
the same file applies to either. Choose the harness when you apply it, with
`--harness`/`-H`, the `SPINLOOP_HARNESS` env var, or a stored default
(`spinloop harness --set`).

## Using a Spinloop

One file, several commands:

- [`spinloop apply`](commands/apply.md) — apply the selection to your agent
- [`spinloop unapply`](commands/unapply.md) — take it back out
- [`spinloop harness -O`](commands/harness.md) — apply it, then launch the agent
- [`spinloop serve`](commands/serve.md) — run `llama-server` for the model it
  names
- [`spinloop alias`](commands/alias.md) — register it under a short name
- [`spinloop export`](commands/export.md) — write one from your current setup

Every command that takes a Spinloop path accepts a directory that holds one and
takes a [registered alias](commands/alias.md) in place of a path. Given no path
at all, it uses the alias `SPINLOOP_ALIAS` names, and failing that `./Spinloop` in
the current directory.

## Fetching a Spinloop from a URL

A Spinloop path can also be an `http://` or `https://` URL, fetched instead of
read from local disk:

```sh
spinloop apply https://example.com/team/Spinloop
```

A URL ending in `/` is treated like a directory — `Spinloop` is appended, so
`spinloop apply https://example.com/team/` fetches
`https://example.com/team/Spinloop`. [`spinloop alias`](commands/alias.md) can
register a URL too, so a team can hand out a short name for a published Spinloop
instead of a link:

```sh
spinloop alias -n team-default https://example.com/team/Spinloop
spinloop apply team-default
```

A relative `PRESET` or `REMOTE` in a URL-sourced Spinloop resolves against that
URL rather than a local directory — see [Syntax](#syntax) below — and is
fetched only when the command that actually needs it runs, never merely
because the Spinloop itself was read.

## Running the model on a cloud GPU

For a model too big for your machine, `REMOTE` names the config of a
scale-to-zero GPU endpoint — one that runs only while you're using it:

```dockerfile
# Spinloop
PROVIDER llamacpp        # the engine to run there, as it would run here
ALIAS    qwen3.6-27b
CONTEXT  131072
PRESET   ./preset.ini
REMOTE   ./remote.json
```

`REMOTE` takes a path, a URL, or a bare name. A path (`./remote.json`, or an
absolute one) is resolved relative to the Spinloop, like `PRESET` — or against
the Spinloop's own URL when it was fetched from one; `REMOTE` may also be an
absolute URL of its own. Either way it is fetched only when a `remote`
subcommand resolves it, or when `apply` falls back to it for the base URL (see
below) — never merely because the Spinloop was read. A bare name
(`REMOTE qwen3.6-27b-prod`) selects a named environment from the per-user
registry at `${XDG_CONFIG_HOME:-~/.config}/spinloop/remotes/<name>/remote.json`,
so deployment state stays per-user and per-instance while only the name lives in
the committed Spinloop — this form is always local, never a URL.
[`spinloop remote`](commands/remote.md) reads whichever it resolves to. With no
path argument the commands consult `./Spinloop` when it exists and otherwise
fall back to the `default` environment; an explicit path
(`spinloop remote status path/to/Spinloop`) requires the Spinloop to carry a `REMOTE`
instruction.

Note the missing `BASEURL`: the endpoint's address belongs to the deployment,
which records it in the named file as `base_url`, and
[`spinloop apply`](commands/apply.md) reads it from there. Write a `BASEURL` only
to override that.

Applying a `REMOTE` Spinloop also names the harness provider after the environment
rather than the engine: the example above is configured under `qwen3.6-27b-prod`,
with the model reading as `qwen3.6-27b-prod/qwen3.6-27b`. `PROVIDER` still
supplies the engine's settings; only the name changes, so several environments
built from the same engine each keep their own entry instead of overwriting one.
The name is the bare `REMOTE` value, or the `environment` field of the file a
path-form `REMOTE` names (falling back to `PROVIDER` when that field is absent).
The provider's display name is qualified by the environment too — `llama.cpp
(qwen3.6-27b-prod)` rather than a bare `llama.cpp` — so a remote environment reads
distinctly from a local engine of the same kind in a harness model picker.

Because `PROVIDER` names the engine, this is the same file that would run the
model locally with [`spinloop serve`](commands/serve.md) — pointed at a bigger
machine.

## Running the model on another machine you own

`FLEET` names a [fleet file](commands/fleet.md#fleetyaml) — the machines on your
network running `spinloop daemon` — and lets `spinloop harness` pick one for you:

```dockerfile
# Spinloop
PROVIDER llamacpp
MODEL    qwen3-27b
FLEET    ./fleet.yaml
```

Launching against it queries the fleet, picks a node already serving that model,
and points the agent at that node's engine. When nothing is serving it, spinloop
starts one and waits for it to load — so the machine you sat down at needs
nothing but a path to the fleet file. `spinloop harness --fleet=<path>` overrides
the instruction, `--node <name>` pins one machine, and `--no-wake` refuses to
start anything.

`FLEET` and `REMOTE` are mutually exclusive: each is a different answer to where
the model is served from, and a Spinloop stating both fails to parse rather than
picking one. As with `REMOTE`, note the missing `BASEURL` — the address is
whichever node gets chosen. Writing one pins the address and turns routing off,
and spinloop says so rather than choosing a node and discarding it.

A `FLEET` may also name a URL rather than a file, for a single endpoint that has
already done the choosing. That is the shape the spinloop gateway will take; it is
not implemented yet, and naming one today fails saying so.

See [`spinloop fleet route`](commands/fleet.md#which-node-would-i-get) to check
which node you would get before launching anything.

## Syntax

One instruction per line: a keyword followed by a single value.

| Keyword    | Required?                  | Maps to        | Example                        |
| ---------- | -------------------------- | -------------- | ------------------------------ |
| `PROVIDER` | yes                              | `--provider`   | `PROVIDER openrouter`          |
| `MODEL`    | one of `MODEL`/`ALIAS`           | `--model`      | `MODEL deepseek/deepseek-v4-pro` |
| `ALIAS`    | one of `MODEL`/`ALIAS`           | `--alias`      | `ALIAS deepseek`               |
| `CONTEXT`  | no                               | `--context`    | `CONTEXT 128k`                 |
| `OUTPUT`   | no                               | `--output`     | `OUTPUT 32k`                   |
| `PARALLEL` | no                               | `spinloop serve`, `spinloop remote deploy` | `PARALLEL 2` |
| `BASEURL`  | no                               | `--base-url`   | `BASEURL https://gateway/v1`   |
| `PRESET`   | no                               | `spinloop serve` | `PRESET ./preset.ini`          |
| `REMOTE`   | no                               | `spinloop remote` | `REMOTE ./remote.json`        |
| `FLEET`    | no                               | `spinloop harness`, `spinloop fleet` | `FLEET ./fleet.yaml` |
| `ENV`      | no (repeatable)                  | `spinloop remote`, `spinloop harness` | `ENV AWS_PROFILE=prod` |

Rules:

- A Spinloop describes **exactly one provider**. `PROVIDER` is required and may
  appear only once; so may every other keyword, except `ENV`.
- You need **at least one** of `MODEL` or `ALIAS`. Give a `MODEL` to add a
  specific model; give an `ALIAS` to name it.
- `MODEL` is the reference the **provider itself** understands: an
  OpenRouter/Bedrock model id, an Ollama name, or — for llama.cpp — a Hugging
  Face repo (`org/model:quant`) or a path to a `.gguf`.
- `ALIAS` is the friendly name the harness shows for the model (and, under
  `serve`, the name `llama-server` reports and the preset section to run). It
  defaults to `MODEL`. For a llama.cpp server the model key is only a label, so
  an `ALIAS` keeps it readable; an `ALIAS` on its own is enough to select one.
- `CONTEXT` sets the context window for the model(s) — always the context a
  single request gets, whatever serves it. It accepts human suffixes (`128k`,
  `1m`) or an absolute count (`200000`).
- `OUTPUT` caps the max output tokens, in the same format as `CONTEXT`. Left
  out, `spinloop` records a quarter of the context. It cannot exceed the context
  window.
- `PARALLEL` sets the number of concurrent request slots for `spinloop serve`
  and `spinloop remote deploy` — a plain integer, not a size. It has no meaning
  for a hosted provider selection, only for a served engine, so unlike
  `CONTEXT`/`OUTPUT` it has no `add`/`remove` CLI flag. Since `CONTEXT` always
  means "context per request", and llama.cpp's own `--ctx-size` is a total
  budget it divides across its `--parallel` slots, a `llamacpp` Spinloop with
  both `CONTEXT` and `PARALLEL` set gets a `--ctx-size` scaled by the slot
  count so each slot still gets what `CONTEXT` promised (`CONTEXT 128k` +
   `PARALLEL 2` → `--ctx-size 256000 --parallel 2`). `vllm`, `mtplx`, and `omlx`
   have no such coupling — `PARALLEL` becomes `--max-num-seqs`/
   `--max-active-requests`/`--max-concurrent-requests` respectively, and
   `CONTEXT` is never scaled by it. See
   [`spinloop serve`](commands/serve.md#parallelism) for the full per-engine
   mapping.
- `BASEURL` overrides the provider's API base URL — handy for a gateway or a
  llama.cpp server on a non-default port. `URL`, `BASE-URL`, and `BASE_URL` are
  accepted as aliases.
- `PRESET` points at a preset `.ini`, used only by
  [`spinloop serve`](commands/serve.md); `apply` ignores it. A relative path
  resolves against the Spinloop's own directory, or against its URL when the
  Spinloop itself was fetched from one; `PRESET` may also be an absolute URL of
  its own, fetched only when `serve` (or `spinloop remote deploy`) builds the
  launch command — never merely because the Spinloop was read. The file is read
  in the flag vocabulary of the engine `PROVIDER` names, so a preset written
  for llama.cpp is not portable to oMLX and vice versa.
- `ENV` sets an environment variable on the machine running `spinloop` and is the
  one keyword that **may repeat**. Its value is a single `KEY=VALUE` token (no
  spaces). The `spinloop remote` commands read it — along with a `.env` beside the
  Spinloop — before they sign their AWS calls, so credentials, region and
  `SPINLOOP_REMOTE_*` overrides can travel with the Spinloop. `spinloop harness` reads
  it too, passing the whole `.env` and the `ENV` lines to the agent it launches.
  Precedence, highest to lowest: an `ENV` line, then a variable already set in
  your shell, then the `.env` — the same rule everywhere spinloop resolves local
  variables. `ENV` applies only on the machine running `spinloop`; it is never sent
  to a deployed instance, and on the harness path it shapes only the launched
  agent, never spinloop's own environment.
- Keywords are **case-insensitive** — `provider`, `Provider`, and `PROVIDER` are
  all accepted — but **UPPERCASE is canonical** and is what `spinloop export`
  writes.
- **Comments** start with `#`, either on their own line or at the end of a line.
  Blank lines are ignored.

To see the available providers, run `spinloop list`. To find a `MODEL` id for one,
run `spinloop list --models <provider>`, which asks the provider's own endpoint
what it currently serves.

## Examples

A local model served by llama.cpp (no API key needed). `ALIAS` is the name
opencode shows; add a `MODEL` (an HF repo or `.gguf` path) or a `PRESET` if you
also want `spinloop serve` to launch it:

```dockerfile
PROVIDER llamacpp
ALIAS    qwen3.6-35b-a3b
```

A single model from OpenRouter (its key comes from your `.env` or
environment, exactly as with `spinloop add`):

```dockerfile
PROVIDER openrouter
MODEL    deepseek/deepseek-v4-pro
```

Any OpenAI-compatible endpoint, with a single pinned model:

```dockerfile
PROVIDER openai-compatible
MODEL    my-model
```

Ready-to-use Spinloops live under [`examples/`](../examples/), including
[`examples/remote-spinloop/`](../examples/remote-spinloop/) for fetching one from
a URL.
