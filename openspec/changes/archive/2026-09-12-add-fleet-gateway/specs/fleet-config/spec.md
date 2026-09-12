## ADDED Requirements

### Requirement: Fleet-wide wake policy

A fleet file MAY declare a top-level `wake` value of `on` or `off`, deciding
whether routing starts an engine on a node that is not running one when no
running node serves what is wanted. It belongs to the file rather than to each
node, for the reason `prefer` does: it describes how this cluster is to be
used — may work be started on its machines on demand, or only used where it is
already running — which is a property of the fleet, not of any one machine in
it.

A file declaring nothing SHALL wake, as routing does when the setting is
absent: waking is the difference between a fleet that answers a request and
one that must be prepared by hand, and the file's author is the one who owns
the machines it names. A file declaring anything other than `on` or `off` SHALL
fail to parse, naming both accepted values, in keeping with the file's other
validation.

The setting SHALL decide whether to wake only. It SHALL NOT change which node
is chosen, how matching nodes are ranked, or what a wake does: a fleet that
declares `wake: off` still reports, when nothing is running, the node whose
source describes the wanted model and the command that would start it.

#### Scenario: A fleet that declares nothing wakes

- **WHEN** a fleet file declares no `wake` setting and routing finds no node
  serving what is wanted
- **THEN** routing starts an engine on a suitable node, as it does today

#### Scenario: A fleet that refuses to wake

- **WHEN** a fleet file declares `wake: off` and routing finds no node serving
  what is wanted
- **THEN** nothing is started, and the failure names the node that would be
  woken and the command that would start it

#### Scenario: An unknown value is rejected at parse time

- **WHEN** a fleet file declares `wake: sometimes`
- **THEN** parsing fails naming `on` and `off`
