# spinloop alias

Register an [`Spinloop` file](../spinloop-file.md) under a short name. The name
then works wherever a Spinloop path does — `apply`, `unapply`, `serve`,
`harness` — from any directory.

```sh
spinloop alias                 # register ./Spinloop under its own ALIAS
spinloop alias path/to/dir     # ...or the Spinloop in another directory
spinloop alias -n big .        # ...under a name of your choosing
spinloop alias --list          # what is registered, and whether it still exists
```

Then, from anywhere:

```sh
spinloop apply big
spinloop serve big
spinloop harness big -- --agent-arg
```

## Registering a URL

The path can be an `http://`/`https://` URL too, so a team can hand out a
short name for a published Spinloop instead of a link:

```sh
spinloop alias -n team-default https://example.com/team/Spinloop
spinloop apply team-default
```

It is stored and resolved just like a local one — see
[Fetching a Spinloop from a URL](../spinloop-file.md#fetching-an-spinloop-from-a-url).

## Naming one for the whole shell

`SPINLOOP_ALIAS` holds a registered name, and any command given no Spinloop uses it:

```sh
export SPINLOOP_ALIAS=big
spinloop apply          # the same as `spinloop apply big`
spinloop serve
spinloop remote status
```

The order is the argument you typed, then `SPINLOOP_ALIAS`, then `./Spinloop`. Two
details worth knowing:

- It decides **which** Spinloop is the default, never **whether** one is applied.
  A bare `spinloop harness` still launches without configuring the agent; `spinloop
  harness -O` asks for the default Spinloop and gets the variable's. And `spinloop
  alias` ignores it entirely — with no argument that command means "the Spinloop
  in this directory".
- It holds a registry name, never a path, and a file of the same name in the
  working directory does **not** shadow it — the opposite of the rule for an
  argument, below. An argument is usually a path; a variable can only have been
  set to name an alias, so it works the same in every directory.

A value that is not registered, or whose Spinloop has gone, fails naming
`SPINLOOP_ALIAS`, so a stale `export` in a shell profile is obvious from the
error.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `-n`, `--name` | Register under this name instead of the Spinloop's `ALIAS` |
| `-F`, `--force` | Re-point a name that is already registered |
| `-l`, `--list` | List the registered names and where they point |

## Two senses of "alias"

They meet here, and they are not the same thing. The `ALIAS` **instruction**
inside a Spinloop names the *model* — it is the key your agent shows, and
`llama-server --alias` under [`serve`](serve.md). A registered **alias** names
the *Spinloop file* to `spinloop`. Registration defaults the second to the first
because you have usually already written a good name; `--name`/`-n` separates
them, and is required when a Spinloop states no `ALIAS` at all.

## Notes

- A real path always beats a registered name, so registering one cannot change
  a command that already worked. When both exist, `spinloop` says so and uses the
  path.
- Names are stored, with absolute paths (a URL is stored as typed), in
  `spinloop`'s own config (`${XDG_CONFIG_HOME:-~/.config}/spinloop/config.json`) —
  they are yours and this machine's, never part of a `Spinloop`, so a committed
  `Spinloop` stays portable.
- Registering parses the Spinloop, so a broken file is caught now rather than
  days later — for a URL, that means fetching it once, at registration time.
- `spinloop alias --list` never makes a network request: a URL-valued alias is
  printed as registered, with no dangling check either way. A local target
  that has since gone is still marked `(missing)`.
- Tab completion knows your aliases — see
  [`spinloop completion`](completion.md).

## See also

- [`spinloop unalias`](unalias.md) — drop a name
- [`spinloop show`](show.md) — lists your aliases alongside the configured state
