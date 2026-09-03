## MODIFIED Requirements

### Requirement: The engine key the client sets

Routing SHALL resolve the engine key from the variable the node's fleet entry
names, supply it to the node when it wakes one, and place it in the launched
agent's environment as `OPENAI_API_KEY` — and, for a harness that reads the key
under its own name, under that name too, as the remote path already does. A key
already set in spinloop's environment SHALL win.

The client is therefore the one party that holds the key: it decides what the
engine it starts is gated with, and it knows what to give the agent because it
set it. A daemon node whose fleet entry names no key SHALL wake an ungated
engine, which is correct for a node reached over loopback.

For a remote node the resolution is the same with a fleet-wide default: the key
is the node's own `engineTokenEnv` when it names one, otherwise the fleet file's
`apiKeyEnv`. A remote's engine is always gated by its API key, so a remote that
is selected — running or woken — SHALL be reached with a key. When neither the
node nor the fleet names a variable, or the variable it names is set nowhere,
routing SHALL fail before the agent launches, naming the node and what to set: a
remote is never reached ungated.

Routing SHALL NOT ask the daemon for a key, and the daemon SHALL NOT return one:
saying a key is required is a fact a router needs, and handing the key out is
not.

Where routing selects a daemon node that is *already running* — one it did not
wake — the engine was gated by whoever started it. When such a node reports that
it requires a key and the fleet entry names none, routing SHALL fail before the
agent launches, naming the node and the variable to set: an agent that cannot
authenticate is worse than a message that says so.

#### Scenario: A woken node is gated with the client's key

- **WHEN** routing wakes a node and its fleet entry names a variable holding a
  key
- **THEN** the node is started gated with that key, and the launched agent's
  environment carries the same value as `OPENAI_API_KEY`

#### Scenario: No key wakes an ungated engine

- **WHEN** routing wakes a daemon node whose fleet entry names no engine key
- **THEN** the engine starts ungated and the agent launches with no key injected
  for it

#### Scenario: A remote node takes the fleet-wide key

- **WHEN** routing selects a remote node whose entry names no `engineTokenEnv`,
  and the fleet file declares an `apiKeyEnv` that is set
- **THEN** the launched agent's environment carries that value as
  `OPENAI_API_KEY`

#### Scenario: A remote node's own key overrides the fleet-wide one

- **WHEN** routing selects a remote node that names its own `engineTokenEnv`
  and the fleet file also declares an `apiKeyEnv`
- **THEN** the node's own variable is the key the agent is given, not the
  fleet-wide one

#### Scenario: A remote node with no key fails early

- **WHEN** routing selects a remote node whose entry names no `engineTokenEnv`
  and the fleet file declares no `apiKeyEnv`
- **THEN** the command fails naming the node and what to set, and no agent is
  launched

#### Scenario: An already-running gated node with no key fails early

- **WHEN** routing selects a daemon node that is already running a gated engine
  and the node's fleet entry names no key
- **THEN** the command fails naming the node and what to set, and no agent is
  launched

#### Scenario: An unset variable fails before anything starts

- **WHEN** a node's fleet entry names an engine key variable that is set nowhere
- **THEN** the command fails naming the node and the variable, and no engine is
  started

#### Scenario: An exported key wins

- **WHEN** `OPENAI_API_KEY` is already set in the user's environment and a
  fleet-routed launch runs
- **THEN** the existing value reaches the agent unchanged
