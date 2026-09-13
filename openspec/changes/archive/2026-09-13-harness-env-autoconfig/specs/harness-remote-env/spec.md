## ADDED Requirements

### Requirement: harness auto-configures from a deployed environment
When `spinloop harness --env <name>` is run with no Spinloop applied — no leading alias or path, and no `--spinloop`/`-O` — the command SHALL fetch the named environment's live environment response (the same fetch that supplies `OPENAI_BASE_URL`/`OPENAI_API_KEY`) and, when it carries a deploy-config, synthesise a provider selection from it rather than doing nothing with the flag.

The deploy-config's runner SHALL become the catalogue provider, by the same mapping `spinloop remote deploy` uses in reverse (a runner is a catalogue provider's engine kind). The deploy-config's served model name SHALL become the model key. The deploy-config's context size, when present, SHALL set the context window. The harness SHALL then be configured and launched exactly as it would be for a Spinloop stating the same `PROVIDER`, `ALIAS` and `CONTEXT` with the same `--env <name>` — the same environment labelling, base URL, and injected credentials as the existing `--env` behaviour.

A Spinloop applied alongside `--env` — a leading alias or path, or `--spinloop`/`-O` — SHALL continue to use its own `PROVIDER`, `ALIAS`, `MODEL` and `CONTEXT` exactly as today; the deploy-config's fields SHALL NOT override a value the Spinloop states.

#### Scenario: bare --env configures and launches the harness
- **WHEN** the user runs `spinloop harness --env dev-3` with no Spinloop applied, and a model is deployed to `dev-3`
- **THEN** the harness is configured with `dev-3` as the provider, the deployed served model name as the model, the deployed context size as the window, and is launched with the fetched base URL and API key in its environment

#### Scenario: an applied Spinloop still wins
- **WHEN** the user runs `spinloop harness some-alias --env dev-3` (or `--spinloop=<path> --env dev-3`) and the Spinloop states its own `PROVIDER` and `ALIAS`
- **THEN** the Spinloop's values configure the harness; the environment's deploy-config is not consulted for them

#### Scenario: trailing args are still forwarded
- **WHEN** the user runs `spinloop harness --env dev-3 --prompt "hello"` with no Spinloop applied, and a model is deployed to `dev-3`
- **THEN** the harness is auto-configured from `dev-3` and launched with `--prompt hello` forwarded to it

### Requirement: harness fails clearly with nothing to auto-configure from
When `spinloop harness --env <name>` is run with no Spinloop applied, and the environment's fetched response carries no deploy-config — because nothing has been deployed to it, or because its `env` Lambda predates this behaviour and the reply simply omits the fields — the command SHALL fail before launching, rather than launch an unconfigured or misconfigured harness.

The error SHALL name the environment and say what to do: deploy a model to it (`spinloop remote deploy <spinloop> --env <name>`), or, when a redeployed model is plausible but the reply still lacks the fields, run `spinloop remote bootstrap` to update the control plane to a version whose `env` Lambda reports what is deployed.

#### Scenario: nothing deployed to the environment
- **WHEN** the user runs `spinloop harness --env dev-3` with no Spinloop applied, and nothing has been deployed to `dev-3`
- **THEN** the command fails saying nothing is deployed to `dev-3` and how to deploy one, and the harness is not launched

#### Scenario: an env Lambda predating this behaviour
- **WHEN** the user runs `spinloop harness --env dev-3` with no Spinloop applied, and `dev-3`'s `env` Lambda reply carries no deploy-config fields
- **THEN** the command fails the same way as when nothing is deployed, naming `spinloop remote bootstrap` as a way to update the control plane, and the harness is not launched
