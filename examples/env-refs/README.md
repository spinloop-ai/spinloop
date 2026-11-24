# Environment references in Spinloop values

A committed `Spinloop` and a shared preset have to work on every machine that
checks them out — but the quant you actually run, the port you keep free, and
the layer count your GPU can take differ per machine. Any value in the file
can reference environment variables — every instruction's value, `PARAM`
lines and preset entries alike — so the file stays machine-neutral and the
command that runs is concrete. This example is the
[qwen3.8-27b setup](../llamacpp/qwen3.8-27b/README.md) pared down to that one
feature; every command here is a dry run — no server or GPU needed.

## 1. The syntax

| Written          | Resolves to                                          |
| ---------------- | ---------------------------------------------------- |
| `$FOO`           | the value of `FOO`; empty when unset or empty        |
| `${FOO}`         | the same, braced — delimits the name, as in `${FOO}1` |
| `${FOO:-fallback}` | `FOO`'s value, or `fallback` when it is unset or empty |
| `$$`             | a literal `$`                                        |

It works in every value: a typed instruction's — `CONTEXT ${CTX:-32768}` — a
`PARAM` value — `PARAM ngl = ${NGL:-99}` — and a preset entry's value —
`port = ${PORT:-8080}`. What does **not** interpolate: keys (preset entry
keys, `PARAM` keys and `ENV` keys are names), and a default's own text, which
is literal — `${FOO:-$BAR}` yields `$BAR` when `FOO` is unset.

## 2. Run it

```sh
cd examples/env-refs
spinloop serve --dry-run
```

```
Using preset preset.ini (model qwen3.8-27b)

llama-server --host 127.0.0.1 --port 8080 --jinja --hf-repo unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL --ctx-size 32768 --n-gpu-layers 99 --alias qwen3.8-27b
```

Nothing is exported, so every reference falls back to its default. Give the
machine its own answers instead:

```sh
QUANT=UD-Q5_K_XL PORT=9100 NGL=40 CTX=65536 spinloop serve --dry-run
```

```
llama-server --host 127.0.0.1 --port 9100 --jinja --hf-repo unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL --ctx-size 65536 --n-gpu-layers 40 --alias qwen3.8-27b
```

The printed command is the resolved command — what runs, and what
`spinloop cloud deploy` would store, resolved the same way before anything
reaches a GPU.

## 3. When references resolve — and when they don't

Resolution happens **where a value is consumed**: when `serve` builds its
command, when `apply` writes the harness config (and `unapply` resolves the
same way to remove exactly what `apply` wrote), when `cloud deploy` derives a
deployment, when a fleet wake starts a node. Reading the files does not resolve
anything: parsing, `spinloop show` and `spinloop harness export` all carry the
raw `$FOO`, so re-rendering a Spinloop never bakes a machine's values — or a
token — into a file.

Whose environment answers depends on the command. Plain `serve` builds from
the environment it inherited — the shell you ran it from. `cloud deploy` and a
node wake apply the Spinloop's own `ENV` instructions and adjacent `.env`
first, so uncommenting the `ENV QUANT=UD-Q5_K_XL` line in [`Spinloop`](Spinloop)
makes that `ENV` line supply what the preset's `${QUANT:-…}` expands to, on
those paths.

## 4. When a reference is malformed

A `$` that merely sits in a value (`$5`, `100%$`) stays literal, and `$$`
escapes one on purpose. Everything else inside `${…}` the syntax doesn't
define fails the command that names it, rather than reaching the engine as raw
text:

```sh
printf 'PROVIDER llamacpp\nMODEL m\nPARAM hf = org/model:${TAG\n' > /tmp/Broken
spinloop serve --dry-run /tmp/Broken
```

```
Error: PARAM hf: unterminated environment reference "${TAG": a `${` reference must end with `}` (write $$ for a literal `$`)
```

A preset's failure names the line instead:

```
Error: /tmp/broken.ini: line 2: unsupported environment reference "${FOO:+x}": only ${NAME} and ${NAME:-default} are resolved (write $$ for a literal `$`)
```

## See also

- [The Spinloop file format](../../docs/spinloop-file.md) — the instructions
  and the environment-reference grammar
- [`spinloop serve`](../../docs/commands/serve.md) — presets, PARAMs and the
  dry run printed above
- [`spinloop cloud deploy`](../../docs/commands/cloud.md) — the same resolution
  on the way to a GPU
- [`examples/llamacpp/qwen3.8-27b`](../llamacpp/qwen3.8-27b/README.md) — the
  full local-plus-AWS walkthrough this setup comes from
