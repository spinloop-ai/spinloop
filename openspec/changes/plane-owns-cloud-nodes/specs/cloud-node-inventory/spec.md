# Spec Delta

## Purpose

Makes the control plane the record of which cloud nodes exist, who
deployed each one, and how one is taken apart, so nothing about a shared
deployment lives only on the machine that made it.

## ADDED Requirements

### Requirement: The control plane enumerates its nodes

The control plane SHALL answer, for a caller it authorises, the set of
nodes deployed on that plane, each with its name, what it is deployed to
serve, its address when it has one, its instance state, and who deployed it and
when. Enumeration SHALL be answered through the control API with the same
credential a control call already uses, so a caller who can start and stop a
node can list them without any further permission.

A node on the plane SHALL be identifiable from that answer alone: a name
the inventory does not carry is a node that does not exist on that
plane, whatever any machine remembers about it.

#### Scenario: Every deployed node is listed

- **WHEN** three nodes are deployed on a plane and its inventory is read
- **THEN** all three are named, each with what it serves and its state

#### Scenario: A deployment made elsewhere is listed

- **WHEN** a node is deployed from one machine and the inventory is read
  from another with credentials for the same plane
- **THEN** that node is named, with the first machine's operator
  recorded as its deployer

#### Scenario: An empty plane lists nothing

- **WHEN** the inventory of a bootstrapped plane with no nodes is read
- **THEN** it answers with no nodes rather than an error

#### Scenario: Enumeration asks for no new permission

- **WHEN** a caller whose credential grants only the usual control-call
  permission requests the inventory
- **THEN** it is answered, and no additional AWS permission is needed

### Requirement: A deploy records who deployed it and when

Each deploy SHALL record with the node the identity of the caller that
deployed it and the time of that deploy, and SHALL report both back to that
caller. A redeploy SHALL record the new deployer and time beside the config it
replaces. Both values SHALL appear in the inventory, so a name that is already
taken can be explained by who took it and what they deployed.

#### Scenario: The deployer is recorded

- **WHEN** an operator deploys a new node
- **THEN** the reply and the inventory both name that operator and the time of
  the deploy

#### Scenario: A redeploy updates the record

- **WHEN** a second operator redeploys that node with `--overwrite`
- **THEN** the inventory credits the second operator and the later time

#### Scenario: A taken name explains itself

- **WHEN** a deploy is refused because the node already exists
- **THEN** the failure names who deployed it, when, and what it serves

### Requirement: cloud ls reports the plane's nodes

`spinloop cloud ls` SHALL list the nodes of the resolved plane — name,
owner, what each serves, address, and state — rather than what this machine
remembers. Where the plane holds no nodes it SHALL say so plainly rather
than printing nothing. Where the plane cannot be resolved or reached it SHALL
fail naming where it looked, and SHALL NOT print a partial or remembered list.

#### Scenario: Listing shows each node with its owner

- **WHEN** two nodes are deployed on a plane by different operators and
  `spinloop cloud ls` runs
- **THEN** both are listed, each with its deployer, what it serves, its address,
  and its state

#### Scenario: A plane with nothing says so

- **WHEN** `spinloop cloud ls` runs against a bootstrapped plane with no
  nodes
- **THEN** the command says there are none, and succeeds

#### Scenario: An unreachable plane is not silently empty

- **WHEN** `spinloop cloud ls` runs with credentials for an account that holds
  no control-plane stack
- **THEN** the command fails naming the stack, account and region it looked in,
  rather than reporting no nodes

### Requirement: A node is taken apart on request

`spinloop cloud undeploy` SHALL end a node by destroying the resources
that belong to it alone — its instance, Elastic IP address, security group,
API-key secret, schedules, and stored deploy configuration — so no allocated
address, open port, secret, or schedule survives a node nobody means to
use again. It SHALL leave the plane's shared resources and every other
node untouched, and it SHALL report what it released. An undeployed
node SHALL deploy again as one, with no residue of the earlier one
holding its name or address.

#### Scenario: Everything node-owned is released

- **WHEN** a node with an instance, an address, a key, a schedule and a
  deploy config is undeployed
- **THEN** the instance is terminated, the address released, the security group
  deleted, the secret deleted, the schedules removed, and the deploy config
  deleted

#### Scenario: Other nodes are untouched

- **WHEN** one node on a plane holding three is undeployed
- **THEN** the other two keep their addresses, keys and schedules, and the
  plane's shared resources remain

#### Scenario: An unknown node is reported as unknown

- **WHEN** `spinloop cloud undeploy --node ghost` names a node the plane
  does not hold
- **THEN** the command fails saying no such node is deployed on that
  plane, and destroys nothing

#### Scenario: The name is free again

- **WHEN** a node is undeployed and that name is then deployed
- **THEN** it deploys as a fresh node, needing its own ingress, and does
  not inherit the earlier deploy config, key, or security group

### Requirement: Undeploy requires explicit consent

Because a node is shared, `spinloop cloud undeploy` SHALL act on exactly
one explicitly named node and SHALL never act on a set: no name SHALL
destroy nothing, and a name given by nothing SHALL be refused rather than
defaulted. Destroying SHALL require explicit consent — a `--yes` flag or an
interactive confirmation — and `--dry-run` SHALL report what would be released
while releasing nothing.

#### Scenario: No name destroys nothing

- **WHEN** `spinloop cloud undeploy` runs with no node named
- **THEN** it fails naming the flag that would say which node, and
  destroys nothing

#### Scenario: Consent is required

- **WHEN** an undeploy is run without consent against an existing node
- **THEN** it stops, naming the node and the flag that confirms it

#### Scenario: A dry run releases nothing

- **WHEN** `spinloop cloud undeploy --dry-run` names an existing node
- **THEN** it lists the resources it would release and contacts no resource for
  deletion
