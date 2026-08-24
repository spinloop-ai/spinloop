## Why

The remote-llms account runs three environments (dev-1, dev-2, dev-3), all on
llama.cpp serving Qwen3.8-27B with MTP. vLLM is a first-class outfit runner —
baked AMI, deploy, serve, metrics, fleet — but it has never actually been run
in one of these environments. Standing up a fourth environment, `vllm-1`, that
mirrors the qwen3.8/MTP set-up on vLLM gives an apples-to-apples comparison of
the two runners on the same model, GPU and context, and tells us whether vLLM
matches or beats the llama.cpp+MTP baseline.

## What Changes

- New `vllm-1/` environment directory in `~/projects/remote-llms`, mirroring
  `dev-1/`:
  - `Outfit`: `PROVIDER vllm`, `MODEL Qwen/Qwen3.8-27B-FP8` (the FP8
    checkpoint, whole-repo seed — no GGUF quant), `ALIAS qwen3.8-27b`,
    `CONTEXT 196608`, `PRESET ./preset.ini`, `REMOTE vllm-1`,
    `ENV AWS_REGION=us-east-1`.
  - `preset.ini` in the vLLM dialect, carrying the flags that mirror
    `dev-1/preset.ini` where vLLM has an equivalent: MTP via
    `speculative-config` (the twin of `spec-type draft-mtp` /
    `spec-draft-n-max 4`), the Qwen recommended thinking-mode sampling
    defaults, and the tool-call parser flags (the twin of `jinja = 1`).
    llama.cpp-only settings (`ngl`, `fa`, `cache-type-k/v`) have no vLLM
    counterpart or use vLLM defaults and are dropped.
- Register `vllm-1` as an outfit alias, `outfit remote deploy` it onto the
  shared layer, and start it. The first start seeds the FP8 checkpoint into
  S3 (`models/vllm/Qwen/Qwen3.8-27B-FP8/`).
- Add `vllm-1` to `fleet.yaml` as a fourth `kind: remote` node, so all four
  environments show in one `fleet status`/`fleet metrics` view.
- Verify and compare against the llama.cpp environments: harness round-trip
  under the same alias, tool calling, MTP acceptance, decode throughput, and
  memory/context fit. Findings are recorded in the remote-llms README.
- Update the remote-llms README: `vllm-1/` in the layout, and the vLLM
  instructions updated from "drop PRESET" to the preset-carrying form this
  environment uses.

Non-goals: changing any existing environment; switching anything to vLLM on
the strength of this test; outfit or CDK project changes (if the test exposes
a gap, that is a separate change); context sizes beyond the mirrored
`196608` (a 262144 probe is a follow-up, not part of this); parallelism
tuning.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None — no outfit requirement changes. `vllm` is already a specified,
first-class runner (`inference-runners`, `local-serving`,
`endpoint-provisioning`), and everything this change does is an instance of
existing behaviour plus files in the separate remote-llms repo. `skip_specs`
is set to `true` for this change.

## Impact

- `~/projects/remote-llms`: new `vllm-1/` (Outfit + preset.ini), `fleet.yaml`
  gains one node, README updated.
- AWS: one more environment on the shared layer (Elastic IP, per-environment
  API key, S3 weights prefix `models/vllm/Qwen/Qwen3.8-27B-FP8/`, one L40S
  instance while running). The vLLM AMI is part of the default bootstrap
  runner set, and the account's shared layer predates the seeding rework, so
  a bootstrap re-run is expected to add the seed-job infrastructure (and to
  bake the vLLM AMI if that is missing too).
- Per-user local state: one alias entry and one
  `~/.config/outfit/remotes/vllm-1/remote.json` (carrying the seed endpoint
  of the shared layer).
- Tooling: the fleet view of a `kind: remote` node and the `outfit remote
  seed` surface need the outfit build from this branch (post-v1.24.2,
  rebased onto main with the seeding rework); the first deploy seeds the
  FP8 checkpoint through the supervised seed job, followed via
  `outfit remote seed status`. The account's shared layer predates the
  seeding rework, so a bootstrap re-run adds the seed infrastructure.
- No outfit code, spec, or CDK change is expected.
