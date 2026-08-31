# Getting started

Install to launched agent, end to end. Five minutes, tops.

## 1. Install

```sh
brew install spinloop-ai/tap/spinloop
```

(Or build from source: `go build -o spinloop ./cmd/spinloop`.)

## 2. See what's on offer

```sh
spinloop list
```

That's the catalogue: every provider, the API key it needs (if any), and which
harnesses support it.

Need a model id to go with a provider? Add `--models` and `spinloop` asks the
provider's own endpoint what it currently serves:

```sh
spinloop list --models openrouter
```

## 3. Configure your agent

Pick a provider and model. For a hosted one, drop the key in a `.env` or
export it first:

```sh
echo 'DEEPSEEK_API_KEY=sk-or-v1-...' > .env
spinloop add -p openrouter -m deepseek/deepseek-v4-flash
```

Or go local — no key needed:

```sh
spinloop add -p ollama -m llama3.2
```

Your agent's existing config survives — `spinloop` merges the provider in,
touching nothing else.

## 4. Launch

```sh
spinloop harness
```

That launches the agent (opencode by default) running the model you picked.
Prefer Pi? `spinloop harness --set pi` once, and every command targets it from
then on.

## 5. Make it declarative

Commit the selection to a file instead of remembering flags. Drop a `Spinloop`
in your project:

```dockerfile
# Spinloop
PROVIDER openrouter
MODEL    deepseek/deepseek-v4-pro
```

Then:

```sh
spinloop apply        # apply it to the agent
spinloop harness -O   # ...or apply and launch in one go
```

Already set up by hand? Capture it: `spinloop export > Spinloop`.

## 6. Serving a local model too?

If the model runs on llama.cpp, the same file launches the server:

```dockerfile
# Spinloop
PROVIDER llamacpp
MODEL    unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL
ALIAS    qwen3.6
CONTEXT  32768
```

```sh
spinloop serve    # runs llama-server for it
spinloop apply    # points the agent at it
```

## 7. Name the ones you keep

```sh
spinloop alias              # registers ./Spinloop under its own ALIAS
spinloop apply qwen3.6      # now the name works anywhere a path does
spinloop harness qwen3.6
```

Wearing one all day? `export SPINLOOP_ALIAS=qwen3.6` and every command that names
no Spinloop uses it — see [`spinloop alias`](commands/alias.md#naming-one-for-the-whole-shell).

## Where next

- [The `Spinloop` file](spinloop-file.md) — full syntax
- [Command reference](README.md#commands) — a page per command
- [Examples](../examples/) — ready-to-apply Spinloops, with walkthroughs
