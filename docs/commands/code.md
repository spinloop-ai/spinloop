# spinloop code

Launch the active harness — one word for `spinloop harness open`. `code` takes
the same flags, applies a `Spinloop` the same way, and forwards the same
trailing arguments, so it is a shortcut for the launch, not a second command:

```sh
spinloop code                            # launch the active harness
spinloop code -O                         # apply ./Spinloop, then launch
spinloop code qwen3.6-27b                # apply the aliased Spinloop, then launch
spinloop code --env prod --prompt "hi"   # configure from prod's deployment, then launch
spinloop code --fleet ./fleet.yaml       # route through a fleet, or its gateway
```

`code` is the only way to launch an agent against a fleet: `spinloop fleet
harness` was removed, and typing it names this command instead. A fleet file
that names a [gateway](../fleet-file.md#gateway) needs no Spinloop at all — the gateway
resolves the model per request — so `spinloop code --fleet ./fleet.yaml` is a
complete launch on a machine holding nothing but the fleet file.

Every flag and argument `code` accepts is [`harness open`'s](harness.md#spinloop-harness-open),
so run `spinloop harness open --help` for the full list, or read that page for
the Spinloop, `--env`, and fleet behaviour in detail.

## See also

- [`spinloop harness open`](harness.md#spinloop-harness-open) — the launch, in full
- [`spinloop up`](up.md) — the one-word start of the engine itself
