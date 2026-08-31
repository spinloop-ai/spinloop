# spinloop export

Print the active harness's configuration as an [`Spinloop` file](../spinloop-file.md),
so you can save a setup you built by hand:

```sh
spinloop export > Spinloop
spinloop export --harness pi > Spinloop   # read Pi's config instead
```

By default it exports the provider behind your default model (or the only
configured provider). If you have several, choose one with `-p`:

```sh
spinloop export -p openrouter > Spinloop
```

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-p`, `--provider` | Which configured provider to export |
| `-H`, `--harness` | Which harness to read (or set `SPINLOOP_HARNESS`) |
| `--providers` | Path to a custom catalogue |

## Notes

- Export names the configured `MODEL` directly.
- It writes canonical UPPERCASE keywords, and records `CONTEXT`/`OUTPUT` only
  when the exported models agree on a value — it never guesses.
- Secrets are never exported; keys stay in your `.env` or environment.

## See also

- [`spinloop apply`](apply.md) — the round trip back
- [`spinloop show`](show.md) — a readable view of the same state
