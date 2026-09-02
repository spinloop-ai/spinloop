## MODIFIED Requirements

### Requirement: Dashboard panels show a health indicator

Each panel SHALL show a coloured status glyph alongside its node's name,
distinct from the border colour that marks the selected panel, so a node's
health reads at a glance across a grid of many panels without reading each
panel's text. The glyph SHALL be shown in every panel shape: a settled
answer, an action in flight, a panel awaiting its first refresh, and a panel
showing a failed outcome.

A node's health SHALL fall into exactly one of five tiers: healthy,
attention, unhealthy, not serving, and unknown. Healthy is coloured green,
attention yellow, and unhealthy red. Not serving and unknown are both
coloured grey and read as "nothing to watch" rather than "something is
wrong"; the two stay apart by their marks — not serving is the same filled
dot as the other tiers in a faded shade, and unknown keeps its `?` — so a
node known to be undeployed never reads as a node the dashboard has not
heard from yet.

- **Healthy**: the node answered its last refresh, its engine is not
  crashed, not `idle`, and not `stopped`, and — when the daemon reports
  readiness for it — the engine is ready. A `running` node whose daemon
  reports no readiness (an older daemon, or a runner with no known health
  check) counts as healthy too, rather than reporting a health tier the
  daemon cannot actually back.
- **Attention**: the node has a start or stop action in flight for it, or is
  `running` with its daemon explicitly reporting the engine not yet ready.
- **Unhealthy**: the node's engine has crashed, or its last refresh's
  outcome was a failure (`unreachable`, `unauthorized`, `config-error`,
  `failed`, or `unsupported`).
- **Not serving**: the node answered its last refresh, no action is in
  flight for it, and its engine is not serving — its state is `idle`,
  nothing has been started, or `stopped`, the state of a stopped daemon
  engine and of a remote environment that has scaled to zero.
- **Unknown**: no status can be determined for the node — it has not yet
  answered any refresh, or its last refresh answered without reporting an
  engine state.

#### Scenario: A running, ready node reads healthy

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports the engine ready
- **THEN** its panel's status glyph is green

#### Scenario: A running node still loading reads attention

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports the engine not yet ready
- **THEN** its panel's status glyph is yellow, even though its engine state
  reads `running`

#### Scenario: A running node with no readiness signal reads healthy

- **WHEN** a node's last completed refresh reports its engine `running` and
  its daemon reports no readiness for it
- **THEN** its panel's status glyph is green, the same as before this
  daemon-side signal existed

#### Scenario: A crashed node reads unhealthy

- **WHEN** a node's last completed refresh reports its engine `crashed`
- **THEN** its panel's status glyph is red

#### Scenario: An unreachable node reads unhealthy

- **WHEN** a node's last refresh could not reach its daemon
- **THEN** its panel's status glyph is red, alongside the outcome and reason
  already shown

#### Scenario: A node awaiting its first refresh reads unknown

- **WHEN** the dashboard opens and a node has not yet answered any refresh
- **THEN** its panel's status glyph is grey, until its first refresh lands

#### Scenario: An answer without a state reads unknown

- **WHEN** a node's last refresh answered but reported no engine state
- **THEN** its panel's status glyph is grey

#### Scenario: An action in flight reads attention

- **WHEN** the operator starts or stops a node and that action has not yet
  finished
- **THEN** that node's panel's status glyph is yellow while the action is in
  flight, whatever the node's last completed refresh reported

#### Scenario: An undeployed node reads not serving

- **WHEN** a node's last completed refresh reports its engine `idle` and no
  start or stop is in flight for it
- **THEN** its panel's status glyph is the faded grey dot, not the green of
  a serving node

#### Scenario: A stopped node reads not serving

- **WHEN** a node's last completed refresh reports its engine `stopped` — a
  daemon engine that was stopped, or a remote environment that has scaled to
  zero — and no start or stop is in flight for it
- **THEN** its panel's status glyph is the faded grey dot, not the green of
  a serving node

#### Scenario: An undeployed node with a start in flight reads attention

- **WHEN** a node's last completed refresh reports its engine `idle` or
  `stopped` and a start for it has not yet finished
- **THEN** its panel's status glyph is yellow while the start is in flight,
  never the grey of not serving

#### Scenario: Not serving and unknown keep different marks

- **WHEN** a grid holds both a node that has answered with `idle` and a node
  that has not yet answered any refresh
- **THEN** the first shows the faded grey filled dot and the second the grey
  `?`, so the two greys are told apart by their mark

#### Scenario: The glyph is distinct from the selection border

- **WHEN** the operator moves the selection onto a panel
- **THEN** the selected panel's border colour changes as it does today, and
  every panel's status glyph colour is unaffected by which panel is selected
