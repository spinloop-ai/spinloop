## ADDED Requirements

### Requirement: MTPLX local provider

The catalogue SHALL include an `mtplx` provider describing a local
[MTPLX](https://mtplx.com) server: an OpenAI-compatible endpoint defaulting to
`http://localhost:8000/v1`, overridable per-provider with `MTPLX_BASE_URL`, and
Pi-capable through the `openai-completions` API.

Like the llama.cpp and oMLX providers it SHALL be optionally authenticated: a
local server needs no key, while the same provider may name an MTPLX server
started with `--api-key` on another machine. An unset key at a local endpoint
therefore yields no `apiKey` option for opencode and the keyless placeholder
for Pi, while a set key — or any non-local endpoint — yields the environment
reference the harness resolves at run time.

Its default port coinciding with another provider's is not a conflict: the
catalogue places no uniqueness requirement on base URLs.

#### Scenario: Local server needs no key

- **WHEN** a selection names `mtplx` with the default localhost base URL and no
  API key set
- **THEN** the opencode provider block carries no `apiKey` option, and the Pi
  entry carries the keyless placeholder so its models stay selectable

#### Scenario: Remote server keeps the key reference

- **WHEN** the base URL is overridden to a non-local host
- **THEN** the API key is written as an environment reference, never as the
  resolved secret
