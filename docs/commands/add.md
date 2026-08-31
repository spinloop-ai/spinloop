# spinloop add

Point your coding agent at a provider and model. Settings are deep-merged into
the agent's config — other providers, your theme, even your comments stay
exactly where you left them.

```sh
spinloop add --provider <name> [--model <id>] [--alias <name>]
           [--context <size>] [--output <size>] [--base-url <url>]
```

## Examples

```sh
# A model from OpenRouter (key from .env or the environment)
spinloop add -p openrouter -m deepseek/deepseek-v4-flash

# A local Ollama model (no key required)
spinloop add -p ollama -m llama3.2

# Claude on AWS Bedrock (uses your AWS credentials)
spinloop add -p amazon-bedrock -m anthropic.claude-3-5-sonnet

# Claude on GCP Vertex AI (uses your Google credentials; set the project)
GOOGLE_VERTEX_PROJECT=my-gcp-project \
  spinloop add -p google-vertex-anthropic -m claude-3-5-sonnet-v2@20241022

# Gemini on GCP Vertex AI
GOOGLE_VERTEX_PROJECT=my-gcp-project \
  spinloop add -p google-vertex -m gemini-2.0-flash

# Any OpenAI-compatible endpoint
OPENAI_API_KEY=sk-... \
  spinloop add -p openai-compatible -m my-model --base-url https://my-endpoint/v1

# Pin a specific default model
spinloop add -p openrouter -m deepseek/deepseek-v4-pro

# Record the context window and cap the output tokens
spinloop add -p llamacpp -m my-model -c 128k -o 32k
```

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-p`, `--provider` | Provider name — see [`spinloop list`](list.md). Required. |
| `-m`, `--model` | The provider-native model id to add or pin as the default |
| `-a`, `--alias` | Friendly name for the model — the key your agent shows |
| `-c`, `--context` | Context window; `128k`, `1m`, `200000`, even `128 K tokens` all work |
| `-o`, `--output` | Max output tokens, same format; defaults to a quarter of the context |
| `-u`, `--base-url` | Override the provider's API base URL (or set `SPINLOOP_BASE_URL`) |
| `-H`, `--harness` | Which harness to configure (or set `SPINLOOP_HARNESS`) |
| `--providers` | Path to a custom catalogue — see [`spinloop init-providers`](init-providers.md) |

## Notes

- You need at least one of `--model` or `--alias` alongside the provider.
- API keys are read from a `.env` beside the `Spinloop` — or, for `spinloop add`,
  which has no Spinloop, from a `.env` in the current directory — then your
  environment, and
  never written anywhere they'll leak. A provider that requires a key tells you
  which variable to set.
- Some cloud providers authenticate with ambient credentials instead of an API
  key: `amazon-bedrock` via your AWS credentials, and `google-vertex` /
  `google-vertex-anthropic` via Google Application Default Credentials (run
  `gcloud auth application-default login`, or set `GOOGLE_APPLICATION_CREDENTIALS`
  to a service-account key file). The Vertex providers need a project — set
  `GOOGLE_VERTEX_PROJECT` (and optionally `GOOGLE_VERTEX_LOCATION`, which
  defaults to `global`). These providers are opencode-only.
- On opencode, `add` sets the chosen model as the default. Pi has no
  default-model setting, so `add` tells you which model to pick with `/model`.
- `--output` needs `--context`, and cannot exceed it.

## See also

- [`spinloop remove`](remove.md) — the inverse
- [`spinloop apply`](apply.md) — the same thing, driven by a `Spinloop` file
- [`spinloop show`](show.md) — what the agent has configured now
