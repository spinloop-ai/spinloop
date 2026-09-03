## ADDED Requirements

### Requirement: Node Spinloop source

A fleet-file node, of either kind, MAY declare a `file` field naming the
Spinloop file that describes what it runs — the same file `spinloop fleet
deploy` reads to create a `kind: remote` node's environment, and the same
file `spinloop fleet start` reads to tell a `kind: daemon` node's engine what
to run. The path SHALL resolve relative to the fleet file's directory, the
same way other Spinloop-relative paths in the project resolve. The field
SHALL NOT be required to parse a fleet file — every fleet command other than
`deploy` and `start` is unaffected by it — but `deploy` and `start` each
SHALL require it (directly or via the fallbacks below) for the nodes they
act on; see fleet-client's "Driving one node" requirement.

#### Scenario: A remote node names its Spinloop file

- **WHEN** a `kind: remote` node declares `file: ./envs/gpu.Spinloop`
- **THEN** `spinloop fleet deploy` for that node reads the Spinloop at that
  path, resolved relative to the fleet file's directory, to derive what to
  deploy

#### Scenario: A daemon node names its Spinloop file

- **WHEN** a `kind: daemon` node declares `file: ./envs/gpu.Spinloop`
- **THEN** `spinloop fleet start` for that node reads the Spinloop at that
  path, resolved relative to the fleet file's directory, to derive what to
  start it with

#### Scenario: The field is inert outside deploy and start

- **WHEN** a node declares a `file` field
- **THEN** `fleet status`, `metrics`, `stop`, `route`, and `dashboard` behave
  exactly as they do without it

### Requirement: Node Spinloop source falls back to name-based lookup

A node declaring no `file` field SHALL have its Spinloop source resolved
from its own `name`, tried in order:

1. `name` resolved as a registered `spinloop alias` — the same lookup a bare
   argument to `spinloop remote deploy <name>` already performs.
2. Failing that, a subdirectory named `<name>` beside the fleet file,
   containing a Spinloop file — the same directory-to-default-file
   resolution an ordinary Spinloop path argument already gets when it names
   a directory.

A node for which neither resolves SHALL fail the command acting on it —
`fleet deploy` for a `kind: remote` node, `fleet start` for a `kind: daemon`
node — for that node alone, naming all three ways a source could have been
given: the `file` field, a `spinloop alias` named after the node, or a
`<name>/` subdirectory beside the fleet file.

#### Scenario: Resolved through a registered alias

- **WHEN** a node named `gpu-env` declares no `file` field, and `spinloop
  alias` has `gpu-env` registered to a Spinloop path
- **THEN** `fleet deploy` (if `gpu-env` is `kind: remote`) or `fleet start`
  (if `kind: daemon`) reads the Spinloop the alias names

#### Scenario: Resolved through a named subdirectory

- **WHEN** a node named `dev-1` declares no `file` field, no alias named
  `dev-1` is registered, and a `dev-1/` directory containing a Spinloop file
  sits beside the fleet file
- **THEN** `fleet deploy` (if `dev-1` is `kind: remote`) or `fleet start` (if
  `kind: daemon`) reads the Spinloop from that subdirectory

#### Scenario: An alias wins over a same-named subdirectory

- **WHEN** a node named `dev-1` declares no `file` field, an alias named
  `dev-1` is registered, and a `dev-1/` subdirectory containing a Spinloop
  file also sits beside the fleet file
- **THEN** the alias is used, not the subdirectory

#### Scenario: None of the three resolve for a remote node

- **WHEN** a `kind: remote` node declares no `file` field, no alias is
  registered under its name, and no same-named subdirectory sits beside the
  fleet file
- **THEN** `fleet deploy` fails for that node, naming the `file` field, the
  alias registry, and the subdirectory convention as the three ways a source
  could have been given

#### Scenario: None of the three resolve for a daemon node

- **WHEN** a `kind: daemon` node declares no `file` field, no alias is
  registered under its name, and no same-named subdirectory sits beside the
  fleet file
- **THEN** `fleet start` fails for that node, naming the `file` field, the
  alias registry, and the subdirectory convention as the three ways a source
  could have been given
