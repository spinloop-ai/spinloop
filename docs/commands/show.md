# spinloop show

Show what a harness currently has configured: its providers, each provider's
models with their context/output limits, the default model, and your registered
aliases.

```sh
spinloop show                # the active harness
spinloop show --harness pi   # a specific one, without changing your default
```

Where [`spinloop list`](list.md) shows the catalogue of providers you *could*
configure, `show` reports what the harness *actually has* right now — and which
config file that lives in.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-H`, `--harness` | Which harness to inspect (or set `SPINLOOP_HARNESS`) |

## Notes

- The output names the active harness and where that choice came from (flag,
  environment, stored preference, or the default).
- Inspecting another harness with `--harness` never touches your stored
  default.

## See also

- [`spinloop export`](export.md) — capture this state as a `Spinloop` file
- [`spinloop alias`](alias.md) — the aliases listed at the bottom
