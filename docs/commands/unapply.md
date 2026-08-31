# spinloop unapply

Remove what an [`Spinloop` file](../spinloop-file.md) selects from your agent's
config — the inverse of [`spinloop apply`](apply.md), just as
[`remove`](remove.md) is to [`add`](add.md).

```sh
spinloop unapply                              # reads ./Spinloop in the current directory
spinloop unapply path/to/Spinloop               # a full path to the file
spinloop unapply path/to/dir                  # a directory holding a Spinloop
spinloop unapply qwen3.6-27b                  # a name registered with `spinloop alias`
spinloop unapply https://example.com/Spinloop   # a URL, fetched instead of read from disk
```

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-H`, `--harness` | Which harness to configure (or set `SPINLOOP_HARNESS`) |
| `--providers` | Path to a custom catalogue |

## Notes

- It honours `--harness`/`-H` and `SPINLOOP_HARNESS` like everything else, so
  unapply from whichever harness you applied to.
- With no argument it resolves the same way `apply` does: `SPINLOOP_ALIAS`, then
  `./Spinloop` — see [`spinloop alias`](alias.md#naming-one-for-the-whole-shell).
- If the agent's default model pointed at something the Spinloop selected, it is
  cleared too.

## See also

- [`spinloop apply`](apply.md) — the other direction
- [`spinloop remove`](remove.md) — the same removal, driven by flags
