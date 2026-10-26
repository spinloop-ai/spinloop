# Qwen3.8-27B on vLLM

Run Qwen3.8-27B with [vLLM](https://docs.vllm.ai) — the engine Qwen's own
Quickstart recommends for production workloads and high-throughput scenarios
— then point opencode at it with the [`Spinloop`](Spinloop) in this directory.
The same file also deploys it to a GPU in AWS with
[`spinloop cloud`](#running-it-on-aws).

## The Spinloop

```dockerfile
PROVIDER vllm
MODEL    Qwen/Qwen3.8-27B-FP8
ALIAS    qwen3.8-27b
CONTEXT  32768
PARAM gpu-memory-utilization = 0.9
```

- `MODEL` is `vllm serve`'s positional argument, not a flag — and the one
  thing a `PARAM` cannot supply. Unlike llama.cpp and MTPLX, vLLM has no
  model flag, so `PARAM hf = …` is not a model to it: `MODEL` is required.
  `MODEL` is passed verbatim, so it may be an HF repo id (fetched by vLLM) or
  a local path.
- `ALIAS` becomes `--served-model-name` — the name opencode shows and asks
  for.
- `CONTEXT` becomes `--max-model-len`. vLLM's context is never scaled by
  `PARALLEL`, so the value is the per-request ceiling as written.
- `PARAM gpu-memory-utilization` is a plain pass-through of an engine flag
  the Spinloop's typed instructions don't cover: the fraction of GPU memory
  vLLM may use for model weights and the KV cache. Add as many `PARAM`s as
  you like — `max-num-seqs`, `enable-chunked-prefill`, a
  `chat-template-kwargs` JSON value, … — each rendered into the command the
  way a preset's flags would be.

## Prerequisites

- [vLLM](https://docs.vllm.ai) on your `PATH` (`pip install vllm`, or the
  engine build your driver needs) — new enough to know the `qwen3_5`
  architecture this model uses.
- A GPU. The FP8 build is roughly 27 GB on disk; 32k of context fits
  comfortably in 24 GB of VRAM, and more headroom buys a larger `CONTEXT`.

## 1. Start vLLM

```sh
spinloop serve              # from this directory; reads ./Spinloop
spinloop serve --dry-run    # print the vllm serve command without running it
```

`spinloop serve` builds the command from the Spinloop, so it runs
equivalently to:

```sh
vllm serve Qwen/Qwen3.8-27B-FP8 \
  --served-model-name qwen3.8-27b \
  --max-model-len 32768 \
  --gpu-memory-utilization 0.9 \
  --port 8000
```

On first run vLLM downloads the weights into its own cache and then starts
serving. The OpenAI-compatible API is at `http://127.0.0.1:8000/v1`.

### Check it's up

```sh
curl http://127.0.0.1:8000/v1/models
```

## 2. Point opencode at it

The `vllm` provider targets that OpenAI-compatible endpoint (default base
URL `http://localhost:8000/v1`). Apply the [`Spinloop`](Spinloop) in this
directory:

```sh
spinloop harness apply examples/vllm/qwen3.8-27b/Spinloop
# or, from this directory:
spinloop harness apply
```

Now start `opencode` and select `vllm/qwen3.8-27b`.

## Running it on AWS

The same Spinloop runs this model on a GPU in the cloud — provisioned by
[`spinloop cloud`](../../../docs/commands/cloud.md) — rather than the machine
in front of you, and terminates itself once you stop using it. This is real,
billed AWS infrastructure (an EC2 GPU instance, an Elastic IP,
image-builder pipelines), so each step below shows you a plan and asks for
confirmation before it creates anything.

```sh
spinloop cloud bootstrap                    # once per AWS account: the control plane
spinloop cloud deploy --env qwen3.8-27b-vllm # from this directory
eval "$(spinloop cloud start --env qwen3.8-27b-vllm)"   # boots (~10 min cold)
spinloop harness open --env qwen3.8-27b-vllm  # work
spinloop cloud stop --env qwen3.8-27b-vllm   # done — shut it down now
```

The deploy reads the same instructions — `MODEL` names the weights, `CONTEXT`
the window — and the PARAMs ride into the server's flags the way they do
locally, the settings the cloud owns dropped. See
[`spinloop cloud`](../../../docs/commands/cloud.md) for the rest: `logs`,
`metrics`, and how to name and switch between multiple deployed
environments.

## See also

- The same model on llama.cpp, with MTP speculative decoding:
  [`examples/llamacpp/qwen3.8-27b-mtp`](../llamacpp/qwen3.8-27b-mtp/README.md)
- The same model on llama.cpp, from a `preset.ini`:
  [`examples/llamacpp/qwen3.8-27b`](../llamacpp/qwen3.8-27b/README.md)
