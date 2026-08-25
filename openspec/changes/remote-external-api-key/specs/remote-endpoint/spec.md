## MODIFIED Requirements

### Requirement: Deploying what the endpoint serves

`spinloop remote deploy` SHALL derive the deployment from the Spinloop and its
preset: `PROVIDER` SHALL select the inference engine, `MODEL` or the preset's
Hugging Face reference SHALL name the weights as a repository and optional
quantisation, `CONTEXT` or the preset's context size SHALL set the window,
`ALIAS` SHALL set the name the endpoint serves under (defaulting to the
repository), and the preset's remaining settings SHALL become the engine's
arguments. Settings the endpoint owns — host, port, model location, API key,
context size, alias, and metrics — SHALL be dropped, so one preset can both
serve locally and deploy unchanged. The request SHALL describe only what to
serve, never where the weights are stored. A `--dry-run` SHALL print the
derived deployment without sending it.

`deploy` SHALL additionally accept a `--api-key-env <VAR>` flag naming the
environment variable that holds the API key to store for the environment. The
flag names a variable, never a literal, and the CLI resolves it from the
process environment before sending it to the control plane to be stored as the
environment's key (see the Environment Deployment specification for how it is
stored and rotated). When a key is supplied, the deploy report SHALL say so
without printing the value.

Deploy SHALL target a named environment: in addition to deriving what to serve,
it SHALL create and register that environment on the control plane (its Elastic
IP, instance configuration, per-environment API key and ingress, and SSM state),
as defined by the Environment Deployment specification. Deploying SHALL NOT start
the instance.

#### Scenario: A preset drives both serving and deploying

- **WHEN** a Spinloop with a preset is deployed
- **THEN** the engine's arguments are the preset's, minus the settings the
  endpoint sets itself

#### Scenario: The Spinloop overrides its preset

- **WHEN** the Spinloop states a `MODEL` and `CONTEXT` that differ from the
  preset's
- **THEN** the Spinloop's values are deployed

#### Scenario: A provider that is not a self-hosted engine

- **WHEN** a Spinloop naming a hosted provider is deployed
- **THEN** the command fails saying that only a self-hosted engine can be
  deployed

#### Scenario: A local model file

- **WHEN** a Spinloop naming a local model file is deployed
- **THEN** the command fails saying to name a repository instead, because the
  endpoint fetches its own weights

#### Scenario: Deploying creates and registers the environment

- **WHEN** a deployment succeeds against a bootstrapped account
- **THEN** the named environment is created and registered in the registry, and
  the report says whether the weights still have to be fetched before it can
  serve

#### Scenario: Deploying is not starting

- **WHEN** a deployment succeeds
- **THEN** the environment is configured but not started, and the report says
  whether the weights still have to be fetched before it can serve

#### Scenario: Deploying with a supplied key

- **WHEN** `spinloop remote deploy` is given `--api-key-env SHARED_KEY` and that
  variable is set
- **THEN** the key is sent to the control plane to be stored for the
  environment, and the report says a key was applied without printing the value
