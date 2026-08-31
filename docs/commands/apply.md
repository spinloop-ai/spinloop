# spinloop apply

Apply an [`Spinloop` file](../spinloop-file.md) — a declarative description of one
provider selection — exactly as if you had run the equivalent
[`spinloop add`](add.md). Everything else in your agent's config is preserved.

```sh
spinloop apply                              # reads ./Spinloop in the current directory
spinloop apply path/to/Spinloop               # a full path to the file
spinloop apply path/to/dir                  # a directory holding a Spinloop
spinloop apply qwen3.6-27b                  # a name registered with `spinloop alias`
spinloop apply https://example.com/Spinloop   # a URL, fetched instead of read from disk
```

Add `--harness pi` (or set `SPINLOOP_HARNESS`) to apply it to Pi instead of
opencode. After applying, just run your coding agent — or do both at once with
[`spinloop harness -O`](harness.md).

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-o`, `--output` | Max output tokens — overrides the Spinloop's `OUTPUT` |
| `-H`, `--harness` | Which harness to configure (or set `SPINLOOP_HARNESS`) |
| `--providers` | Path to a custom catalogue (a Spinloop never names one) |

## Notes

- With no argument, `apply` uses the alias `SPINLOOP_ALIAS` names, and failing
  that a file named `Spinloop` in the current directory — see
  [`spinloop alias`](alias.md#naming-one-for-the-whole-shell).
- A URL ending in `/` is treated like a directory — `Spinloop` is appended. See
  [Fetching a Spinloop from a URL](../spinloop-file.md#fetching-an-spinloop-from-a-url).
- A Spinloop's `PRESET` line is for [`spinloop serve`](serve.md); `apply` ignores
  it — never fetched, even when it's a URL.
- A Spinloop with a `REMOTE` line and no `BASEURL` takes the endpoint's address
  from that [remote config](remote.md)'s `base_url`, which its deployment
  writes. A `BASEURL` in the Spinloop wins over it, and a remote config that
  isn't there yet is not an error — apply just leaves the base URL alone.

## See also

- [The `Spinloop` file](../spinloop-file.md) — full syntax
- [`spinloop unapply`](unapply.md) — the inverse
- [`spinloop export`](export.md) — write a Spinloop from your current setup
