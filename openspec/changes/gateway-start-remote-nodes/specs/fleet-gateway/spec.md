## MODIFIED Requirements

### Requirement: Listing the fleet's models

The gateway SHALL serve `GET /v1/models` returning, in the OpenAI list shape,
the union of the models a request can reach. For each node whose state is
`running`, the list SHALL carry the name it reports serving — the served name
when it reports one, otherwise the model id. For each node that is not
running and is a wake candidate — waking is allowed for it (its own `wake`
setting, or the fleet-wide one when it names none) and it names a model to
start with — the list SHALL additionally carry that model, under the same
served-name-first naming a wake would start it with: a daemon node's own
Spinloop source describes it, and a remote node's own stats reply carries it
— the environment's stored deploy config, which the stats reply carries
whether the environment is running or stopped (unlike the status reply,
which only relays it while running). A running node SHALL contribute nothing
but what it reports: a running engine is never displaced, so its source's
model is not a request the gateway would answer from it. An undeployed remote
environment — one whose stats read fails outright, having no deploy config
to read — and a node for which waking is not allowed SHALL contribute
nothing beyond what is running, and duplicates SHALL be listed once. The
model a node would be started with SHALL be resolved at most once in a short
window shared by all models requests, so a burst does not re-read every
node's source or re-fetch every remote node's stats.

#### Scenario: Running models are listed

- **WHEN** two nodes are running, one serving a model under an alias and one
  under its id, and a models request is made
- **THEN** the response lists the alias and the id, each once

#### Scenario: A stopped node's wakeable model is listed

- **WHEN** a node is stopped, its own source describes a model, and waking is
  allowed for it, and a models request is made
- **THEN** the response lists the model the source describes, beside what the
  running nodes serve

#### Scenario: A stopped node's model is not listed when wake is off

- **WHEN** a node is stopped and waking is not allowed for it, and a models
  request is made
- **THEN** the response lists only what the running nodes serve

#### Scenario: A deployed remote environment's model is listed

- **WHEN** a remote environment is stopped, its stats reply reports what its
  stored deploy config would serve, and waking is allowed for it, and a
  models request is made
- **THEN** the response lists that model beside what the running nodes serve

#### Scenario: A stopped remote environment's model is not listed

- **WHEN** a remote environment is stopped and has nothing deployed, and a
  models request is made
- **THEN** the response does not list it: the gateway has nothing stored to
  start it with

#### Scenario: A running node's source adds no second model

- **WHEN** a running node reports one model and its source describes another,
  and a models request is made
- **THEN** the response lists only what the node reports

#### Scenario: A burst of models requests resolves each source once

- **WHEN** several models requests arrive within the window in which a node's
  source or a remote node's status is resolved
- **THEN** each node's source or status is read once for the burst

#### Scenario: Nothing reachable lists nothing

- **WHEN** no node is running and nothing is wakeable — no stopped node
  describes a model, or waking is not allowed for any of them — and a models
  request is made
- **THEN** the response is an empty list, not an error

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
- A remote node names one through its own stats reply, which reads the
  environment's stored deploy config directly and so carries its model id
  whether the environment is running or stopped — unlike its status reply,
  which only relays the deploy config while running, and unlike the stats
  reply itself, which carries no served name. An undeployed remote
  environment's stats read fails outright, having no deploy config to read;
  it names nothing and is not a candidate.

A node is started with what it names, never with a config invented for the
request: a daemon node is started with the Spinloop source's config; a remote
node is started as it is — its stored deploy config decides what it serves,
and the gateway pushes it nothing new. Candidates whose stored config already
names the model SHALL be tried first, since they have the weights, and the
rest in fleet-file order. A node that refuses the start — a runner or model it
cannot serve — SHALL NOT fail the request while other candidates remain.

A daemon engine started this way SHALL be gated with the key the node's fleet
entry names, supplied by the gateway: the gateway is the client that starts
the engine, so the key the client sets is the key the engine takes. A remote
environment's engine is gated by its own key, resolved the same way a request
already routed to it resolves one; the gateway does not change it. The wait
SHALL be bounded by a wake timeout, defaulting to five minutes and
overridable by `--wake-timeout`; exceeding it SHALL fail the request saying
the engine did not answer in time, and the started engine SHALL be left
running rather than stopped, so a slow load — or, for a remote node, a slow
boot — is not thrown away.

When several requests ask for a model nothing is serving at once, the gateway
SHALL start at most one engine per node and answer every request from it: the
first request's wait is the wait the rest join. A node another request woke
first SHALL be used the same way, and only once its engine answers.

A request for a model nothing is serving, and for which waking is not allowed
on any node that names it, SHALL fail without starting anything, naming the
nodes and what they could serve, and the command that would start one. A
model no node is running and no node names — no daemon source describes it
and no remote node is deployed with it — SHALL fail the same way regardless
of any wake setting: nothing to wake with, and the failure SHALL say so
rather than trying to start a node with nothing.

#### Scenario: A cold request wakes a node and is served

- **WHEN** no node is running the model a request names, one node's Spinloop
  source describes it, and waking is allowed for it
- **THEN** that node is started with its own config, gated with the key its
  fleet entry names, and the request is answered once the engine answers

#### Scenario: A cold request wakes a deployed remote environment

- **WHEN** no node is running the model a request names, one remote node's
  stats reply reports it is deployed to serve it, and waking is allowed for
  it
- **THEN** that environment's instance is started, its own stored deploy
  config decides what it serves, and the request is answered once its engine
  answers

#### Scenario: An undeployed remote node is not a wake candidate

- **WHEN** the only node whose name could match a request is a remote
  environment with nothing deployed
- **THEN** it is not started, and the failure says nothing is deployed to
  serve the model, naming the deployment path

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

#### Scenario: A remote node opted out is not woken though the fleet wakes

- **WHEN** the fleet file's wake policy is `on`, a stopped remote node
  declares its own `wake: off`, and it is the only node that names the
  model a request asks for
- **THEN** it is not started, and the failure names it and says waking is
  disabled for that node

#### Scenario: Nothing can serve the model

- **WHEN** no node is running the model a request names, no node's Spinloop
  source describes it, and no remote node is deployed with it
- **THEN** the request fails, naming each node and why it cannot serve the
  model, and nothing is started

#### Scenario: A sourceless node is not woken

- **WHEN** the only node that could take a request names no Spinloop source
  that resolves
- **THEN** it is not started, and the failure names it and the ways a source
  could have been given

### Requirement: Serving the fleet's topology

The gateway SHALL serve the fleet's topology over a read endpoint, behind the
same caller authentication as everything else it serves: a caller with the
token gets it, a caller without does not, exactly as on its other paths.

The reply SHALL describe each node of the fleet the gateway holds: its name,
its kind, its tags as the fleet file declares them, its state, and its
serving facts — the model it serves when it is running, the name it serves
that model under where it reports one, whether its engine has answered, and
when it was last active. For a node that is not running, the reply SHALL name
the model a request would start it with, where the node names one — a daemon
node's own source, or a remote node's own stats reply — and waking is
allowed for it (its own `wake` setting, or the fleet's when it names none); a
node that names no such model, or for which waking is not allowed, SHALL
report none. A node that does not answer SHALL be reported as such in the
fleet's order, not fail the whole reply.

The reply SHALL carry the fleet file's fleet-level settings the way the file
declares them: whether the fleet wakes, how it ranks, and its concurrency
limits where it declares any, with each absent where the file declares it
not. The gateway's fleet file remains the single source of truth for the
topology: the reply is what the file says and what the nodes report, and the
gateway holds no copy of either beyond what it already holds.

#### Scenario: A caller with the token reads the topology

- **WHEN** a caller sends the gateway's token to the topology endpoint
- **THEN** it gets every node with its tags, state, and serving facts, and
  the file's wake policy, ranking, and concurrency limits

#### Scenario: A caller without the token is refused

- **WHEN** a caller sends no token, or the wrong one, to the topology
  endpoint
- **THEN** it is refused the way the gateway's other paths refuse it

#### Scenario: A stopped node reports what it would start

- **WHEN** a node is not running, names a model to start with, and waking is
  allowed for it
- **THEN** the topology names that model as what a request would start the
  node with

#### Scenario: A stopped, deployed remote node reports what it would start

- **WHEN** a remote node is stopped, its stats reply reports its stored
  deploy config, and waking is allowed for it
- **THEN** the topology names that config's model as what a request would
  start it with, the same way a daemon node's is named

#### Scenario: A dead node does not sink the reply

- **WHEN** one of the fleet's nodes does not answer and a caller reads the
  topology
- **THEN** that node is reported as not answering, in the fleet's order, and
  the rest of the fleet is reported as usual

#### Scenario: Absent settings are absent

- **WHEN** the fleet file declares no concurrency limits
- **THEN** the topology carries none, rather than a default the file never
  named
