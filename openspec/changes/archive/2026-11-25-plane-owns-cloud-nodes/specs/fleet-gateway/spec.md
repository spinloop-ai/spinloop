# Spec Delta

## MODIFIED Requirements

### Requirement: Waking a node for a request

When no running node serves the model a request names, and waking is allowed
for at least one candidate, the gateway SHALL start an engine on a node that
is not running one, and SHALL hold the request until the engine answers.
Waking is allowed for a node when its own `wake` setting says so, or,
when it names none, the fleet file's wake policy does. A node is a wake
candidate when it is not running, waking is allowed for it, and it names a
model to start with, matching the one the request asks for:

- A daemon node names one through the Spinloop source it names — its `file`
  field, a registered alias named after it, or a same-named directory beside
  the fleet file, resolved the way `spinloop fleet start` resolves it —
  describing a config whose model or served name is the one the request asks
  for.
- A cloud node names one through its own stats reply, which reads the
  node's stored deploy config directly and so carries its model id
  whether the node is running or stopped — unlike its status reply,
  which only relays the deploy config while running, and unlike the stats
  reply itself, which carries no served name. Whether such a node
  exists at all SHALL be settled by the plane's inventory of deployed
  nodes rather than inferred from that read failing: a cloud node whose
  node the plane does not hold names nothing and is not a candidate,
  and a node whose node the plane does hold stays a candidate even when
  a particular read of it fails for an unrelated reason.

A node is started with what it names, never with a config invented for the
request: a daemon node is started with the Spinloop source's config; a cloud node
node is started as it is — its stored deploy config decides what it serves,
and the gateway pushes it nothing new. Candidates whose stored config already
names the model SHALL be tried first, since they have the weights, and the
rest in fleet-file order. A node that refuses the start — a runner or model it
cannot serve — SHALL NOT fail the request while other candidates remain.

A daemon engine started this way SHALL be gated with the key the node's fleet
entry names, supplied by the gateway: the gateway is the client that starts
the engine, so the key the client sets is the key the engine takes. A cloud node
node's engine is gated by its own key, resolved the same way a request
already routed to it resolves one; the gateway does not change it. The wait
SHALL be bounded by a wake timeout, defaulting to five minutes and
overridable by `--wake-timeout`; exceeding it SHALL fail the request saying
the engine did not answer in time, and the started engine SHALL be left
running rather than stopped, so a slow load — or, for a cloud node, a slow
boot — is not thrown away.

When several requests ask for a model nothing is serving at once, the gateway
SHALL start at most one engine per node and answer every request from it: the
first request's wait is the wait the rest join, regardless of the node's
kind. A daemon node's own control API refuses a second concurrent start on
its own, but a cloud node's control plane does not, so the gateway
SHALL NOT rely on that alone: two requests racing to wake the same node
SHALL be coalesced before either reaches the node, not just reconciled after
one of them answers. A node another request woke first SHALL be used the
same way, and only once its engine answers.

A request for a model nothing is serving, and for which waking is not allowed
on any node that names it, SHALL fail without starting anything, naming the
nodes and what they could serve, and the command that would start one. A
model no node is running and no node names — no daemon source describes it
and no cloud node is deployed with it — SHALL fail the same way regardless
of any wake setting: nothing to wake with, and the failure SHALL say so
rather than trying to start a node with nothing.

#### Scenario: A cold request wakes a node and is served

- **WHEN** no node is running the model a request names, one node's Spinloop
  source describes it, and waking is allowed for it
- **THEN** that node is started with its own config, gated with the key its
  fleet entry names, and the request is answered once the engine answers

#### Scenario: A cold request wakes a deployed cloud node

- **WHEN** no node is running the model a request names, one cloud node's
  stats reply reports it is deployed to serve it, and waking is allowed for
  it
- **THEN** that node's instance is started, its own stored deploy
  config decides what it serves, and the request is answered once its engine
  answers

#### Scenario: An undeployed cloud node is not a wake candidate

- **WHEN** the only node whose name could match a request is a cloud node
  naming a node the plane's inventory does not carry
- **THEN** it is not started, and the failure says nothing is deployed to
  serve the model, naming the deployment path

#### Scenario: An existing node is a candidate though a read failed

- **WHEN** a cloud node's node is in the plane's inventory as deployed to
  serve the model a request names, but its stats read failed because the caller's
  credentials were rejected on that call
- **THEN** the node is still considered a candidate, and the failure the caller
  sees is the rejected credential rather than "nothing is deployed to serve it"

#### Scenario: The request is held while the engine loads

- **WHEN** the woken node reports running while its engine is still loading
- **THEN** the request waits, and is answered when the engine answers, rather
  than failing against an endpoint that refuses connections

#### Scenario: A wake that does not finish in time fails the request

- **WHEN** a woken node's engine does not answer within the wake timeout
- **THEN** the request fails saying so, naming the node, and the engine is left
  running

#### Scenario: Concurrent cold requests share one wake

- **WHEN** two requests arrive at once for a model nothing is serving, and one
  node's source describes it
- **THEN** that node is started once, and both requests are answered from the
  same engine

#### Scenario: Concurrent cold requests share one cloud node wake

- **WHEN** two requests arrive at once for a model nothing is serving, and one
  cloud node's stats reply reports it is deployed to serve it
- **THEN** that node's instance is started once, not once per request,
  and both requests are answered once its engine answers

#### Scenario: A woken engine takes the gateway's key

- **WHEN** the gateway starts an engine on a daemon node whose fleet entry
  names an engine key
- **THEN** the engine is gated with that value, the gateway's requests to it
  carry it, and no reply to any caller contains it

#### Scenario: Wake refused by the fleet file

- **WHEN** waking is not allowed for any node that could serve the model a
  request names, and no node is serving it
- **THEN** nothing is started, and the request fails naming the node whose
  source describes the model and the command that would start it

#### Scenario: A cloud node opted out is not woken though the fleet wakes

- **WHEN** the fleet file's wake policy is `on`, a stopped cloud node
  declares its own `wake: off`, and it is the only node that names the
  model a request asks for
- **THEN** it is not started, and the failure names it and says waking is
  disabled for that node

#### Scenario: Nothing can serve the model

- **WHEN** no node is running the model a request names, no node's Spinloop
  source describes it, and no cloud node is deployed with it
- **THEN** the request fails, naming each node and why it cannot serve the
  model, and nothing is started

#### Scenario: A sourceless node is not woken

- **WHEN** the only node that could take a request names no Spinloop source
  that resolves
- **THEN** it is not started, and the failure names it and the ways a source
  could have been given

