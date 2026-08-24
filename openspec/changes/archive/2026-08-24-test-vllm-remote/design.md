## Context

See proposal.md for motivation. Current state that shapes the approach:

- The shared layer and the three `dev-N` environments in
  `~/projects/remote-llms` all run llama.cpp serving Qwen3.8-27B with MTP
  (`spec-type draft-mtp`, `spec-draft-n-max 4`), `CONTEXT 196608`, the Qwen
  thinking-mode sampling defaults, `jinja = 1` for tool calling, and a q8 KV
  cache.
- `vllm` is already a first-class, specified outfit runner, and the work is
  merged to main: baked AMI (vLLM 0.26.0 in a venv, engine booted with
  `--gpu-memory-utilization 0.92`, the API key delivered through a
  `VLLM_API_KEY` env file rather than a key file), whole-checkpoint seeding
  (sentinel `config.json`), deploy/serve/fleet/metrics/log paths.
- In a cloud deployment the *only* route by which user flags reach the engine
  is the preset: `deployConfigFor` flattens the chosen preset section (plus
  `[*]`) into `serveArgs`, dropping the cloud-owned set (host, port, model
  source, key, ctx-size, alias, metrics, companion paths, the parallelism
  keys). The `VLLM` preset dialect passes long-form keys through unchanged,
  and the parser already handles JSON-looking values (it splits on the first
  `=`, and `#`/`;` only start a comment after whitespace).
- The fleet view of `kind: remote` nodes needs the outfit build from this
  branch (five commits past v1.24.2); deploy/start/status work on any recent
  build.
- Weight seeding was reworked on main into a supervised job (today): a real
  seeder program streams Hugging Face → S3 on a stock AL2023 box (no GPU
  image, no bake), reports phase/progress to CloudWatch, self-terminates, and
  writes a `_seed.json` manifest as its final step (which also replaces the
  vLLM runner's `config.json` sentinel — the vLLM seed selection is still the
  whole checkpoint). Deploying with absent weights starts a seed and returns a
  seed *handle*; `outfit remote seed start|status|ls|stop` follows it, and a
  wake before the seed reaches `succeeded` would sync an incomplete prefix, so
  the first start waits on the seed. The account's shared layer was
  bootstrapped before this rework, so it needs a bootstrap re-run to gain the
  seed Lambda, log group and seeder bundle (a new environment's remote.json
  then carries `seed_url`).

## Goals / Non-Goals

**Goals:**

- A working `vllm-1` environment that mirrors `dev-1` as closely as the two
  engines allow, so a harness pointed at either addresses the same model name.
- Comparable, recorded data: decode throughput, MTP acceptance, tool calling,
  memory/context fit, side by side with the llama.cpp environments in one
  fleet view.

**Non-Goals:**

- No outfit or CDK project changes (a gap found in testing becomes a separate
  change).
- No changes to the existing `dev-N` environments.
- No context above the mirrored 196608 (a 262144 probe is a follow-up), and
  no parallelism tuning.

## Decisions

**1. Same `ALIAS`, different `REMOTE`.** `vllm-1` serves under
`qwen3.8-27b`, exactly like the dev environments. The harness's model request
stays identical; the only variable between runners is the endpoint (base URL
+ key). A distinct alias would change the model name the agent asks for and
add a second variable to the comparison. Environments are told apart by the
remote name, which is already how the dev environments coexist.

**2. `MODEL Qwen/Qwen3.8-27B-FP8`, whole checkpoint.** vLLM serves the synced
checkpoint directory; the deploy config carries no quant for vllm, and the
seed downloads the whole snapshot. FP8 is the documented choice for this model
in both READMEs and the native dtype for the GPU. The GGUF quant the dev
environments use has no vLLM equivalent — the comparison is FP8 vs Q8_K_XL,
which is inherent to the runners and recorded as such.

**3. `CONTEXT 196608`, mirrored.** The dev contexts were capped at 196608 by
llama.cpp's MTP draft-buffer headroom; vLLM's MTP state is much smaller, so
this should fit with headroom. Mirroring keeps the comparison fair — jumping
straight to 262144 would confound runner differences with context-size
differences. A 262144 probe is a follow-up once the mirror is green.

**4. MTP, sampling and tool calling go in a vLLM preset.** The README's
"drops PRESET" vLLM form cannot express MTP, and the preset is the only flag
route to a cloud engine (and, per this repo's convention, the same file also
runs locally under `outfit serve`). Draft `vllm-1/preset.ini`:

```ini
; vLLM preset for Qwen3.8-27B. The vllm dialect passes long-form keys through
; unchanged; boolean flags take an empty value (a value would reach the
; engine and break argparse's store_true).

[*]
; Qwen's recommended thinking-mode sampling defaults, server-side. opencode
; sends its own per request; these cover clients that don't. In step with
; dev-1.
temperature        = 1.0
top-p              = 0.95
top-k              = 20
min-p              = 0.0
repetition-penalty = 1.0
presence-penalty   = 0.0

[qwen3.8-27b]
; MTP: the model's multi-token-prediction module drafts tokens that vLLM
; verifies in one pass — the vLLM twin of spec-type draft-mtp with
; spec-draft-n-max 4.
speculative-config = {"method": "mtp", "num_speculative_tokens": 4}
; Tool calling: structured tool_calls out of the model's output — the vLLM
; twin of jinja = 1.
enable-auto-tool-choice =
tool-call-parser      = qwen3_coder
```

Notes:

- `speculative-config` renders as two argv tokens (`--speculative-config`
  plus the JSON); the engine builds argv from a string slice, so no shell
  quoting is involved.
- `num_speculative_tokens` starts at 4 to mirror `spec-draft-n-max 4`. vLLM
  accepts it (the MTP module's `n_predict` is 1, so the divisibility check
  passes) but warns that >1 runs the single MTP layer multiple times per step
  "which may result in lower acceptance rate". If acceptance looks poor at 4,
  2 (or 1) is the documented fallback — a verification outcome, not a
  redesign.
- The parser name is `qwen3_coder`, not `qwen3`: vLLM 0.26.0's registry has no
  bare `qwen3` entry — `qwen3_coder`, `qwen3_xml` and `mimo` all map to the
  same `Qwen3EngineToolParser`. The checkpoint is a `qwen3_5` VLM wrapper
  (`Qwen3_5ForConditionalGeneration`) with `mtp_num_hidden_layers: 1` under
  `text_config`, so the live tool-calling check (task 4.1) remains the
  authority on which registered name matches its template.
- `gpu-memory-utilization` is deliberately *not* in the preset: the runner
  spec already sets 0.92 in `extraServeArgs`, and the preset is not dropped
  for this flag, so a second value would reach the engine and win.
- llama.cpp-only settings are dropped, not translated: `ngl` (a GPU engine
  offloads by definition), `fa` (flash attention is vLLM's default),
  `cache-type-k/v` (see decision 5).

**5. KV cache left at the default.** No `--kv-cache-dtype` in the baseline:
the model's KV footprint is small (a hybrid SSM/attention model where only a
fraction of layers grow a KV cache, per the dev-1 comment), and L40S is Ada —
no native FP8 tensor cores — so an fp8 KV cache would be a software path of
unverified cost/benefit. If 196608 does not fit at 0.92 memory,
`kv-cache-dtype fp8` is the first knob to add, noted in the comparison.

**6. `PARALLEL` left unset.** llama.cpp's MTP forced the dev environments to
one slot; vLLM's speculative decoding runs under continuous batching, so
vllm-1 may serve concurrent requests. Leaving it at vLLM's default makes that
a *finding* of the test rather than a tuned setting.

**7. Fleet node.** `vllm-1` joins `fleet.yaml` as a fourth `kind: remote`
node, keeping "one fleet, side-by-side rows" the comparison surface
(`fleet status`, `fleet metrics -w`). `fleet start` on a remote node wakes
the environment through its start Lambda (only the route-with-a-new-config
form is refused for remote nodes).

**8. Tooling.** Use an outfit built from this branch, *after* the rebase onto
main, for everything (`go build -o outfit ./cmd/outfit`): the fleet
remote-node view and the `outfit remote seed` surface both need the new
commits, and one binary keeps deploy/start/fleet behaviour consistent.

## Risks / Trade-offs

- **vLLM 0.26.0 may not register an MTP speculative method for this model
  family** → the engine log at first start shows the spec-decoder decision
  (`outfit remote logs vllm-1`); if absent, vllm-1 runs without
  `speculative-config` and the comparison records the gap as a finding
  rather than failing.
- **The tool-call parser name may differ in vLLM 0.26.0** (or be absent for
  this family) → verify with a curl'd tool-calling request before the harness
  test; check the parser registry on the AMI's vLLM for the right name; if
  none exists, structured tool calls degrade to raw text, which is itself a
  reportable answer to "can we use vLLM here".
- **Memory fit at 196608** (FP8 checkpoint + MTP state + default KV at 0.92
  of 48 GB) → an engine that fails to start or OOMs is visible in the logs;
  knobs in order: `kv-cache-dtype fp8`, then a lower context — each noted in
  the comparison.
- **Wrong or missing HF repo name** → verify `Qwen/Qwen3.8-27B-FP8` exists
  before deploying; a failed seed fails loudly with the name, but the first
  seed is a ~28 GB whole-checkpoint copy into S3, so a typo costs a long job.
- **Long first seed** (whole FP8 checkpoint, ~28 GB, Hugging Face → S3) →
  expected; it is now a supervised job: `outfit remote seed status` reports
  phase and progress, a seed whose box dies is reported failed rather than
  stuck in progress, and a 60-minute cap bounds its life. The instance pull
  and model load still follow the seed.
- **Billing while up** → the environment scales to zero on the idle timer;
  keep the comparison window deliberate, and `outfit remote stop`
  terminates when the test is done for the day.
- **Preset boolean idiom is easy to break** (`flag = 1` reaches the engine
  and breaks argparse) → the booleans above are written with empty values;
  a local `outfit serve vllm-1 --dry-run` check catches a regression before
  anything deploys.
- **README drift** → the remote-llms README currently documents the
  preset-less vLLM form; this change updates it to the preset-carrying form
  so the two cannot diverge.

## Migration Plan

Additive only; nothing existing changes:

1. Add `vllm-1/` to remote-llms (Outfit + preset.ini), update `fleet.yaml`
   and the README; commit in that repo.
2. `outfit alias -n vllm-1 vllm-1/Outfit`; `outfit remote deploy vllm-1`
   (starts the seed, returns its handle); follow `outfit remote seed status`
   to `succeeded`; `outfit remote start vllm-1`; verify and compare.

Rollback: `outfit remote stop vllm-1` (terminate), `outfit unalias vllm-1`,
remove `~/.config/outfit/remotes/vllm-1/`, revert the remote-llms commit. The
S3 seed can stay (harmless, reusable) or be deleted. No shared-layer change is
made by this environment; the one exception is a bootstrap run if the account
predates the vLLM AMI bake — that is itself additive and idempotent.

## Open Questions

- Whether the live model output actually parses under `qwen3_coder` (the
  registry check passed — `qwen3_coder`/`qwen3_xml`/`mimo` are the
  Qwen3-family entries in vLLM 0.26.0, all the same `Qwen3EngineToolParser` —
  but the template match is the live request's to confirm in task 4.1).
- Whether `num_speculative_tokens 4` shows good acceptance (the engine accepts
  it; the MTP layer's `n_predict` is 1, so 4 runs the layer four times per
  step, which vLLM warns may lower acceptance — fallback 2 or 1).
- Whether 262144 fits on vLLM once the mirror is green — deliberately a
  follow-up, kept out of this change to protect the comparison.
