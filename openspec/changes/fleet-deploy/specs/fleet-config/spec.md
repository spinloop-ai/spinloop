## ADDED Requirements

### Requirement: Remote node deploy source

A `kind: remote` node MAY declare a `file` field naming the Spinloop file
that node's environment is deployed from — the same file `spinloop fleet
deploy` reads to derive what that node serves. The path SHALL resolve
relative to the fleet file's directory, the same way other Spinloop-relative
paths in the project resolve. The field SHALL NOT be required to parse a
fleet file, since every other fleet command drives an already-deployed
environment and has no use for it; it is consulted only by `fleet deploy`. A
`kind: daemon` node declaring a `file` field SHALL have it ignored — not
because a daemon node has no notion of a Spinloop file describing what it
serves (routing already wakes an idle daemon node with a deploy config
derived from the Spinloop being launched), but because this field feeds only
`fleet deploy`, which persistently creates the cloud environment a `kind:
remote` node addresses — a step a daemon node's already-provisioned machine
has no equivalent of.

#### Scenario: A remote node names its Spinloop file

- **WHEN** a `kind: remote` node declares `file: ./envs/gpu.Spinloop`
- **THEN** `spinloop fleet deploy` for that node reads the Spinloop at that
  path, resolved relative to the fleet file's directory, to derive what to
  deploy

#### Scenario: The field is inert outside deploy

- **WHEN** a `kind: remote` node declares a `file` field
- **THEN** `fleet status`, `metrics`, `start`, `stop`, `route`, and
  `dashboard` behave exactly as they do without it

#### Scenario: Ignored on a daemon node

- **WHEN** a `kind: daemon` node declares a `file` field
- **THEN** parsing succeeds and the field has no effect on that node

### Requirement: Remote node deploy source falls back to name-based lookup

A `kind: remote` node declaring no `file` field SHALL have its deploy source
resolved from its own `name`, tried in order:

1. `name` resolved as a registered `spinloop alias` — the same lookup a bare
   argument to `spinloop remote deploy <name>` already performs.
2. Failing that, a subdirectory named `<name>` beside the fleet file,
   containing a Spinloop file — the same directory-to-default-file
   resolution an ordinary Spinloop path argument already gets when it names
   a directory.

A node for which neither resolves SHALL fail `fleet deploy` for that node
alone, naming all three ways a source could have been given: the `file`
field, a `spinloop alias` named after the node, or a `<name>/` subdirectory
beside the fleet file.

#### Scenario: Resolved through a registered alias

- **WHEN** a `kind: remote` node named `gpu-env` declares no `file` field,
  and `spinloop alias` has `gpu-env` registered to a Spinloop path
- **THEN** `fleet deploy` for that node reads the Spinloop the alias names

#### Scenario: Resolved through a named subdirectory

- **WHEN** a `kind: remote` node named `dev-1` declares no `file` field, no
  alias named `dev-1` is registered, and a `dev-1/` directory containing a
  Spinloop file sits beside the fleet file
- **THEN** `fleet deploy` for that node reads the Spinloop from that
  subdirectory

#### Scenario: An alias wins over a same-named subdirectory

- **WHEN** a `kind: remote` node named `dev-1` declares no `file` field, an
  alias named `dev-1` is registered, and a `dev-1/` subdirectory containing a
  Spinloop file also sits beside the fleet file
- **THEN** `fleet deploy` for that node reads the Spinloop the alias names

#### Scenario: None of the three resolve

- **WHEN** a `kind: remote` node declares no `file` field, no alias is
  registered under its name, and no same-named subdirectory sits beside the
  fleet file
- **THEN** `fleet deploy` fails for that node, naming the `file` field, the
  alias registry, and the subdirectory convention as the three ways a source
  could have been given
