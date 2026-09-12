## Why

A harness that runs against a fleet today learns the gateway's address from a
`FLEET` instruction in a Spinloop — but the Spinloop is the node-level
artifact (what a node runs), and the fleet's nodes already name their
Spinloops. Having a Spinloop name a fleet or a gateway closes that circle: one
artifact doing two jobs. The gateway is a property of the fleet, and the
command that points a harness at a fleet belongs at the fleet's level of the
hierarchy.

## What Changes

- The fleet file MAY name its gateway: an optional top-level `gateway` section
  beside `wake:` and `prefer:` carrying the gateway's `url` and, optionally,
  a `tokenEnv` naming the variable that holds its token.
- New `spinloop fleet harness` command: the fleet-level form of a
  fleet-routed launch. It takes a Spinloop the way `spinloop harness` takes
  one and a fleet file from `--fleet`/`-f` (defaulting to the `fleet.yaml`
  beside it), routes the launch at the gateway the file names — or, where the
  file names none, to a node with the existing selection and wake — applies
  the Spinloop, and launches the harness.
- Routing precedence generalises the existing rule: an explicit `-f` wins
  over a Spinloop's `FLEET`; with no `-f`, the Spinloop's `FLEET` (file or
  endpoint) is used; with neither, `./fleet.yaml` is read. A pinned `BASEURL`
  and an exported `OPENAI_BASE_URL` still win over any routing.
- `spinloop fleet route` answers a Spinloop whose fleet file names a gateway:
  it reports the gateway's address, queries no node, and wakes nothing.
- `examples/gateway-docker/` moves its client onto the command: the example's
  fleet file gains a `gateway` section, the client Spinloop loses its `FLEET`
  URL, and the integration test exercises the new path.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fleet-config`: a fleet file MAY declare a top-level `gateway` section
  naming the address the fleet is served under and the variable holding its
  token.
- `fleet-routing`: a launch whose effective fleet file names a gateway is
  routed at that gateway (the section's address as the base URL, its token
  variable resolved through the client's key chain, no node contacted);
  `fleet route` answers such a file by naming the gateway.
- `fleet-client`: a `spinloop fleet harness` command — the fleet-level way to
  configure and launch the harness against a fleet.

## Impact

- The fleet file's config package: the `gateway` section's parsing.
- `cmd/spinloop`: the new `fleet harness` subcommand, built on the launch
  path's routing rather than re-implementing it; `fleet route`'s endpoint
  answer gains the file-with-a-section case.
- `examples/gateway-docker/`: the client Spinloop's `FLEET` URL becomes a
  `gateway` section plus the command; `run-tests.sh` asserts the new flow.
- `docs/`: the section in the fleet file documentation;
  `docs/commands/gateway.md` naming the command as the way to point a harness
  at a fleet.
- Additive: a `FLEET` instruction keeps working exactly as it does today —
  its removal is a separate, later change. No daemon endpoint changes, no new
  dependencies.
