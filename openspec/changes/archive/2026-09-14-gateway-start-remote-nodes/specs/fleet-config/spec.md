## MODIFIED Requirements

### Requirement: Fleet-wide wake policy

A fleet file MAY declare a top-level `wake` value of `on` or `off`, deciding
whether routing starts an engine on a node that is not running one when no
running node serves what is wanted. It applies to every node that declares
no `wake` setting of its own — the same reason `prefer` is fleet-wide: it
describes how this cluster is to be used — may work be started on its
machines on demand, or only used where it is already running — which is a
property of the fleet, not of any one machine in it, unless that machine's
entry says otherwise.

A node entry MAY declare its own `wake` value, `on` or `off`, in the same
shape as the fleet-wide setting. When a node names one, it decides whether
that node may be woken, taking precedence over the fleet-wide setting for
that node alone; a node naming none is governed by the fleet-wide setting as
before. This exists because waking is not free the same way on every node: a
remote environment's wake boots and pays for a cloud instance, unlike a local
daemon's engine, so an operator may want the fleet's daemons to wake freely
while deciding a remote node's waking on its own terms — opted in under a
fleet that otherwise does not wake, or opted out under one that does —
without a second fleet-wide flag governing every remote node in the file
alike.

A file declaring nothing at either level SHALL wake, as routing does when no
setting decides otherwise: waking is the difference between a fleet that
answers a request and one that must be prepared by hand, and the file's
author is the one who owns the machines it names. A fleet-wide or per-node
`wake` declaring anything other than `on` or `off` SHALL fail to parse,
naming both accepted values, in keeping with the file's other validation.

The setting SHALL decide whether to wake only, at whichever level decides it
for a given node. It SHALL NOT change which node is chosen, how matching
nodes are ranked, or what a wake does: a node for which waking is not
allowed still reports, when nothing is running, the node whose source
describes the wanted model and the command that would start it.

#### Scenario: A fleet that declares nothing wakes

- **WHEN** a fleet file declares no `wake` setting at either level and
  routing finds no node serving what is wanted
- **THEN** routing starts an engine on a suitable node, as it does today

#### Scenario: A fleet that refuses to wake

- **WHEN** a fleet file declares `wake: off` and no node overrides it, and
  routing finds no node serving what is wanted
- **THEN** nothing is started, and the failure names the node that would be
  woken and the command that would start it

#### Scenario: A node opts out under a fleet that wakes

- **WHEN** a fleet file declares `wake: on`, one node declares its own
  `wake: off`, and that node is the only one serving what is wanted
- **THEN** that node is not woken, and the failure names it and says waking
  is disabled for it, even though the fleet otherwise wakes

#### Scenario: A node opts in under a fleet that does not wake

- **WHEN** a fleet file declares `wake: off`, one node declares its own
  `wake: on`, and that node is the only one serving what is wanted
- **THEN** that node is woken, though the rest of the fleet still does not
  wake

#### Scenario: An unknown value is rejected at parse time

- **WHEN** a fleet file declares `wake: sometimes`, at the fleet level or on
  a node
- **THEN** parsing fails naming `on` and `off`
