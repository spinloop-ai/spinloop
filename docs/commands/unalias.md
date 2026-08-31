# spinloop unalias

Drop a name registered with [`spinloop alias`](alias.md). The `Spinloop` file it
pointed at is left alone — only the name goes away.

```sh
spinloop unalias qwen3.6-27b
```

## Notes

- Takes exactly one name; see `spinloop alias --list` for what's registered.
- With [tab completion](completion.md) set up, `spinloop unalias <TAB>` offers
  exactly the names you have.

## See also

- [`spinloop alias`](alias.md) — register or re-point a name
