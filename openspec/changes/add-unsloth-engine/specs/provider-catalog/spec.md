## ADDED Requirements

### Requirement: Unsloth Studio local provider

The catalogue SHALL include an `unsloth` provider describing a local
[Unsloth Studio](https://unsloth.ai) server: an OpenAI-compatible endpoint
defaulting to `http://localhost:8888/v1`, overridable per-provider with
`UNSLOTH_BASE_URL`, and Pi-capable through the `openai-completions` API.

Its key SHALL be read from `UNSLOTH_API_KEY` and SHALL be optional: a local
endpoint with the key unset yields no `apiKey` option for opencode and the
keyless placeholder for Pi.

#### Scenario: Local server with keyless access needs no key

- **WHEN** a selection names `unsloth` with the default localhost base URL and
  `UNSLOTH_API_KEY` unset
- **THEN** the opencode provider block carries no `apiKey` option, and the Pi
  entry carries the keyless placeholder so its models stay selectable

#### Scenario: The Studio key is referenced, never written

- **WHEN** `UNSLOTH_API_KEY` is set
- **THEN** the API key is written as a reference to `UNSLOTH_API_KEY` that the
  harness resolves at run time, never as the resolved secret

#### Scenario: A remote Studio keeps the key reference

- **WHEN** the base URL is overridden with `UNSLOTH_BASE_URL` to a non-local
  host
- **THEN** the API key is written as an environment reference, never as the
  resolved secret
