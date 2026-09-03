## ADDED Requirements

### Requirement: Remote node deploy source

A `kind: remote` node MAY declare a `spinloop` field naming the Spinloop file
that node's environment is deployed from — the same file `spinloop fleet
deploy` reads to derive what that node serves. The path SHALL resolve
relative to the fleet file's directory, the same way other Spinloop-relative
paths in the project resolve. The field SHALL NOT be required to parse a
fleet file, since every other fleet command drives an already-deployed
environment and has no use for it; it is consulted only by `fleet deploy`. A
`kind: daemon` node declaring a `spinloop` field SHALL have it ignored — the
field describes what a *remote* environment is deployed from, and a daemon
node's machine is the operator's own.

#### Scenario: A remote node names its Spinloop file

- **WHEN** a `kind: remote` node declares `spinloop: ./envs/gpu.Spinloop`
- **THEN** `spinloop fleet deploy` for that node reads the Spinloop at that
  path, resolved relative to the fleet file's directory, to derive what to
  deploy

#### Scenario: The field is inert outside deploy

- **WHEN** a `kind: remote` node declares a `spinloop` field
- **THEN** `fleet status`, `metrics`, `start`, `stop`, `route`, and
  `dashboard` behave exactly as they do without it

#### Scenario: Ignored on a daemon node

- **WHEN** a `kind: daemon` node declares a `spinloop` field
- **THEN** parsing succeeds and the field has no effect on that node
