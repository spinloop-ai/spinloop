# Remove the FLEET instruction from the Spinloop file

## Why

The `FLEET` instruction duplicates a fact the directory already holds. A
Spinloop that sits beside its `fleet.yaml` does not need to name it, and a
Spinloop named from elsewhere is being pointed at deliberately — an incidental
`fleet.yaml` in the working directory should not hijack that launch. The
gateway feature (a `gateway` section in the fleet file) already gives a fleet
an address its members do not have to carry, so the instruction's remaining
job — naming where the model is served from — is covered by the directory
convention plus the `--fleet` flag.

Support goes away entirely: a Spinloop that still carries `FLEET` fails to
parse with the standard unknown-keyword error, which lists the accepted
keywords. There is no migration message and no compat path.

## What changes

- **Spinloop grammar** (`spinloop-files`): `FLEET` is no longer a keyword; the
  parser rejects it as unknown. The `FLEET and REMOTE are exclusive` and
  `FLEET names a file or an endpoint` requirements are removed, along with the
  parse-time exclusivity check and the `FLEET` line `Format` emits. `REMOTE`
  stays — its removal is a separate follow-up.
- **Routing discovery** (`fleet-routing`): a launch routes through the fleet
  file given by `--fleet`/`-f` when that is passed. Otherwise, when the
  Spinloop is not named explicitly — no positional path, no `--spinloop` value,
  no `SPINLOOP_ALIAS` — a `./fleet.yaml` in the working directory is used. An
  explicitly named Spinloop routes only when `--fleet` is given. A pinned
  `BASEURL` still wins over routing, as today.
- **`spinloop fleet route`** (`fleet-client`): the Spinloop's `FLEET` is no
  longer a fleet source; the same discovery rules apply, and a route with no
  fleet file in force fails naming `--fleet`.
- **`spinloop fleet harness`** (`fleet-client`): the fleet file comes from
  `--fleet`/`-f` or `./fleet.yaml` beside the command; the Spinloop's `FLEET`
  is no longer consulted.
- **FLEET-as-endpoint removed**: the endpoint form of `FLEET` (a URL value,
  added by the still-open `add-fleet-gateway` change) is removed with the rest
  of the instruction — its code lands on this same branch, unarchived.
- **`spinloop gateway` output**: the address it prints is named in a fleet
  file's `gateway` section rather than in a Spinloop's `FLEET`.
- **`add-fleet-gateway` delta trimmed**: that open change's delta specs are
  edited so archiving it after this one cannot resurrect FLEET requirements —
  its `spinloop-files` delta goes, its two endpoint-`FLEET` `fleet-routing`
  requirements go, and its `fleet-gateway` output wording points at the
  `gateway` section.
- **Docs and examples**: the `FLEET` sections of the Spinloop file and command
  docs are replaced with the discovery rules; `examples/fleet-local` drops
  `FLEET` from its Spinloop (it sits beside its `fleet.yaml`), and
  `examples/fleet-docker`'s client Spinloop drops `FLEET` with the test run
  passing `--fleet`.

## Capabilities

- `spinloop-files`: `Spinloop file format` modified (keyword list, scenarios);
  `FLEET and REMOTE are exclusive` removed; `FLEET names a file or an endpoint`
  removed.
- `fleet-routing`: `A fleet-routed launch` removed and restated as `A launch
  routed through a fleet` (discovery rules; its FLEET scenarios cannot stand
  under them); `A fleet file naming a gateway routes the launch at it` modified
  (its fleet file no longer comes from the Spinloop's `FLEET`).
- `fleet-client`: `Explaining a route` modified; `A fleet harness command`
  removed and restated as `The fleet harness command` (fleet sources).
- `fleet-config`: `A fleet file MAY name a gateway` modified (wording: the
  `FLEET` it referenced is gone).
