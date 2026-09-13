# spinloop provider

Work with the provider catalogue — the set of providers `spinloop` can
configure a harness with.

```sh
spinloop provider list   # the catalogue: every provider, its key, which harnesses support it
spinloop provider init   # write the built-in catalogue out as a file you can edit
```

## spinloop provider list

Show the catalogue: every provider `spinloop` can configure, the API key each
one needs (if any), and which harnesses support it.

```sh
spinloop provider list
spinloop provider list --providers ./my-providers.yaml   # a custom catalogue
spinloop provider list --models                           # add each provider's live models
spinloop provider list --models openrouter                # just one provider's models
```

This is the *could* view — what's available to configure. For what your agent
*currently has* configured, see
[`spinloop harness show`](harness.md#spinloop-harness-show).

The catalogue names providers, not models — models change too often to curate.
`--models` fills that gap on demand: it asks each provider's own endpoint what
it currently serves (OpenRouter, an OpenAI-compatible gateway, a local llama.cpp
or Ollama server, …) and lists the ids you can drop into a `Spinloop`'s `MODEL`.
A positional provider name narrows the query to one provider.

| Flag | Meaning |
| ---- | ------- |
| `--providers` | Path to a custom catalogue (or set `SPINLOOP_PROVIDERS`) — see [`spinloop provider init`](#spinloop-provider-init) |
| `--models` | Also fetch each listed provider's current models, live from its endpoint |

Notes:

- A provider marked with a required API key won't configure until that
  variable is set in a `.env` beside your `Spinloop`, or in your environment.
- Not every provider maps to every harness — the listing names which harnesses
  each supports (AWS Bedrock, for instance, is opencode-only).
- `--models` is best-effort: it uses the same base URL and key a selection
  would, applies a short timeout, and prints `(none found)` when a provider is
  unreachable or has no queryable endpoint (AWS Bedrock has none). A plain
  `spinloop provider list` makes no network request.

## spinloop provider init

Write the built-in provider catalogue out as a file you can edit — the starting
point for adding your own providers without rebuilding.

```sh
spinloop provider init                 # writes ./providers.yaml
spinloop provider init custom.yaml     # ...or to a path of your choosing
```

Edit it, then point `spinloop` at it — the flag wins, then the env var, then the
built-in default:

```sh
spinloop provider list --providers ./providers.yaml
SPINLOOP_PROVIDERS=./providers.yaml spinloop provider list
```

| Flag | Meaning |
| ---- | ------- |
| `-F`, `--force` | Overwrite an existing file |

Notes:

- It refuses to overwrite an existing file unless `--force` is given, so a
  stray run can't destroy a catalogue you've been editing.
- The written file is commented with the schema — providers, key environment
  variables, endpoints, and per-harness settings are all data, not code.

## See also

- [`spinloop harness add`](harness.md#spinloop-harness-add) — configure something you found here
- [`spinloop harness show`](harness.md#spinloop-harness-show) — what the harness has configured
