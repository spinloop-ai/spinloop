# Design

## What is removed, and what stays

The `FLEET` instruction goes away in both forms — a fleet-file path and a
gateway URL (the latter added by the still-open `add-fleet-gateway` change,
whose code lands on this same branch). That deletes:

- `Selection.Fleet`, the `kwFleet` parse case, the REMOTE/FLEET exclusivity
  check, `FleetIsEndpoint`, and the `FLEET` line in `Format`.
- The endpoint branch of `routeThroughFleet` and `fleet route`'s endpoint
  handling.
- Every FLEET scenario in the four affected requirements.

What stays:

- The `gateway` section of the fleet file (`add-fleet-harness`, already
  archived) — the fleet's address now lives with the fleet.
- `endpointBaseURL` and the gateway-section routing branch of
  `routeThroughFleet`: the section branch points a launch at the fleet's
  gateway exactly as the old endpoint branch did, and both need the same
  base-URL normalisation.
- `REMOTE` and everything around it. Its removal is a separate follow-up;
  only the exclusivity check that paired it with `FLEET` goes.

## The discovery rule, per command

"Explicitly named" means the user supplied the identity of the Spinloop the
launch wears: a leading positional argument, a `--spinloop`/`-O` value, or
`SPINLOOP_ALIAS` set. A valueless `--spinloop` wears the default Spinloop and
counts as not named — the user asked for whatever is there, which is exactly
the case where a `fleet.yaml` beside it is the local convention. A bare
`spinloop harness` wears no Spinloop at all (the documented "still applies
nothing"), so there is nothing to route and the discovery rules never engage.

| command | fleet file in force |
| --- | --- |
| `spinloop harness` | `--fleet`/`-f`; else `./fleet.yaml` when the Spinloop was not explicitly named; else none |
| `spinloop fleet route` | `--fleet`/`-f`; else `./fleet.yaml` when the Spinloop was not explicitly named; else fail naming `--fleet` |
| `spinloop fleet harness` | `--fleet`/`-f`; else the `fleet.yaml` beside the command — the Spinloop's explicitness does not gate this one, because its fleet file has always come from the command, never from beside the Spinloop |

A pinned `BASEURL` still beats routing on every path, and the stderr
announcement ("Using … at … — …") is unchanged in shape.

`fleet route` failing with no fleet file in force is the one behaviour that
gets stricter: it used to error naming the missing `FLEET`, and now errors
naming `--fleet` — same shape, different fix.

## Restated requirements, not modified ones

OpenSpec refuses to drop a scenario from a `MODIFIED` requirement, and two of
the requirements being changed carry scenarios whose names reference the
`FLEET` ("The flag overrides the instruction", "A Spinloop with no FLEET is
unaffected", "The command's file wins over the Spinloop's FLEET"). Leaving
those names in the main spec would keep the removed keyword on the page, so the
two requirements are removed and restated under clean names, the same
remove+add-under-a-new-name pattern `retire-model-families` used:

- `fleet-routing`: `A fleet-routed launch` → `A launch routed through a fleet`.
- `fleet-client`: `A fleet harness command` → `The fleet harness command`.

Prose elsewhere that speaks of "a fleet-routed launch" as a concept rather
than naming the requirement is left alone; the only formal cross-references are
updated to the new names.

## No fail-loud migration

A Spinloop carrying `FLEET` now fails with the standard unknown-keyword error,
which lists the accepted keywords. A dedicated "FLEET is gone, do X instead"
message would be a migration branch in the parser for a keyword that never
comes back, and the standard error plus the docs carry the same information.

## Why the `add-fleet-gateway` delta is trimmed here

`add-fleet-gateway` is unarchived on this branch, and its delta adds the
endpoint-`FLEET` behaviour this change deletes. If the delta stayed intact,
archiving it after this one would re-add requirements whose code no longer
exists. The trim is part of this change so the two archive in a consistent
order on main:

- `specs/spinloop-files/spec.md` — deleted outright; its only requirement is
  the endpoint form of `FLEET`.
- `specs/fleet-routing/spec.md` — the two endpoint-`FLEET` ADDED
  requirements go; the MODIFIED `Choosing a node` and `Waking a node` stay
  untouched (neither mentions FLEET).
- `specs/fleet-gateway/spec.md` — the gateway command's printed address is
  named in a fleet file's `gateway` section rather than a Spinloop's `FLEET`,
  in the requirement text and its scenario.

## Examples

`examples/fleet-local`: the Spinloop sits beside its `fleet.yaml`, so dropping
the `FLEET` line keeps `spinloop harness` routing exactly as the example
describes — the README's story sharpens rather than changes.

`examples/fleet-docker`: the client's Spinloop lives in a subdirectory, so an
explicit `-O` path would no longer pick up the fleet. The Spinloop drops its
`FLEET` line and the test run passes `--fleet` explicitly — which is also a
live demonstration of the flag overriding the directory convention.

`examples/gateway-docker` needs no change to its Spinloop (it carries no
`FLEET`); its test run is checked against the new `fleet harness` resolution.
