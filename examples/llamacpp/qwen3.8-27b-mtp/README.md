# Qwen3.8-27B with MTP speculative decoding on llama.cpp

Run Unsloth's GGUF build of Qwen3.8-27B with llama.cpp's multi-token
prediction (MTP) speculative decoding — the draft head sits inside the GGUF, so
no separate draft file is downloaded — then point opencode at it with the
[`Spinloop`](Spinloop) in this directory. The same file also deploys it to a
GPU in AWS with [`spinloop cloud`](#running-it-on-aws).

What makes this example different from its
[`qwen3.8-27b`](../qwen3.8-27b/README.md) sibling: there is no
`preset.ini`. The engine options live in the Spinloop itself, one `PARAM` per
flag:

```dockerfile
PROVIDER llamacpp
ALIAS    qwen3.8-27b
CONTEXT  131072
PARAM hf = unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL
PARAM ngl = 99
PARAM fa = on
PARAM jinja
PARAM spec-type = draft-mtp
PARAM spec-draft-n-max = 2
```

Each `PARAM` names one engine flag — a `key = value` pair, or a bare key where
the flag takes no value — rendered into the `llama-server` command in
llama.cpp's own dialect — `fa` becomes `--flash-attn`, `ngl` becomes
`--n-gpu-layers`, and so on. `PARAM`
may repeat, and a PARAM stands over a preset when both set the same flag —
a Spinloop can mix the two. Where the Spinloop states an instruction of its
own (`MODEL`, `CONTEXT`, `PARALLEL`, …), the instruction wins over a PARAM.

`PARAM hf` names the model in place of a `MODEL`: `serve` (and `cloud
deploy`) read it the way they read a preset's `hf`, so a Spinloop with no
preset file at all still deploys what it serves.

## Prerequisites

- A recent build of [llama.cpp](https://github.com/ggml-org/llama.cpp) that
  includes `llama-server` **and MTP speculative decoding**
  (`--spec-type draft-mtp`) (e.g. `brew install llama.cpp`, or build from
  source) — new enough to know the `qwen3_5` architecture this model uses.
- A GPU is strongly recommended. The `UD-Q5_K_XL` quant is roughly 20 GB on
  disk; for comfortable headroom plan for ~24 GB of VRAM (less if you
  offload fewer layers to the GPU).

## 1. Start llama-server

```sh
spinloop serve              # from this directory; reads ./Spinloop
spinloop serve --dry-run    # print the llama-server command without running it
```

`spinloop serve` builds the command from the Spinloop's PARAMs, so it runs
equivalently to:

```sh
llama-server \
  -hf unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL \
  --jinja -ngl 99 --flash-attn on \
  --ctx-size 131072 \
  --spec-type draft-mtp --spec-draft-n-max 2 \
  --host 127.0.0.1 --port 8080
```

On first run `llama-server` downloads the `UD-Q5_K_XL` weights into the
llama.cpp cache (`~/.cache/llama.cpp`) and then starts serving.

What the PARAMs do:

- `hf` — model repository and quant tag, fetched from Hugging Face.
- `ngl = 99` — offload all layers to the GPU. Lower it (or drop it) for
  CPU-only or limited VRAM.
- `fa = on` — flash attention, which also keeps the quantised KV cache an
  option (see the sibling example's [KV-cache section](../qwen3.8-27b/README.md#optional-quantise-the-kv-cache)
  if 128k does not fit).
- `jinja` — use the model's built-in chat template, written bare because the
  flag takes no value. Required for Qwen3 tool calling to work correctly.
- `spec-type = draft-mtp` — speculative decoding with the model's own
  multi-token-prediction head. Worth roughly 1.5–2× on decode;
  `llama-server` logs the acceptance rate.
- `spec-draft-n-max = 2` — how many draft tokens the head proposes per step.
- `CONTEXT 131072` — the context window in tokens, rendered as `--ctx-size`.
  The model natively supports 262144, so raise it on a box with the memory.

MTP does not support parallel sequences, so there is deliberately no
`PARALLEL` here: one request at a time is the engine's contract, not an
oversight.

### Check it's up

```sh
curl http://127.0.0.1:8080/v1/models
```

## 2. Point opencode at it

`llama-server` speaks the OpenAI-compatible API, which is exactly what the
`llamacpp` provider targets (default base URL `http://localhost:8080/v1`).
Apply the [`Spinloop`](Spinloop) in this directory:

```sh
spinloop harness apply examples/llamacpp/qwen3.8-27b-mtp/Spinloop
# or, from this directory:
spinloop harness apply
```

`ALIAS` is the name opencode shows for the model. For a single-model server
it's just a label — `llama-server` serves whichever model it loaded
regardless of what's requested — so call it whatever you find readable.
`CONTEXT` matches opencode's context window to the `--ctx-size` the server
was launched with, so it doesn't overshoot what `llama-server` will accept.

Running on a non-default host or port? Add a `BASEURL` line to the Spinloop
(the file ships one commented out):

```dockerfile
BASEURL http://127.0.0.1:9090/v1
```

Now start `opencode` and select `llamacpp/qwen3.8-27b`.

## Running it on AWS

The same Spinloop runs this model on a GPU in the cloud — provisioned by
[`spinloop cloud`](../../../docs/commands/cloud.md) — rather than the machine
in front of you, and terminates itself once you stop using it. This is real,
billed AWS infrastructure (an EC2 GPU instance, an Elastic IP,
image-builder pipelines), so each step below shows you a plan and asks for
confirmation before it creates anything.

```sh
spinloop cloud bootstrap                     # once per AWS account: the control plane
spinloop cloud deploy --env qwen3.8-27b-mtp  # from this directory
eval "$(spinloop cloud start --env qwen3.8-27b-mtp)"   # boots (~10 min cold)
spinloop harness open --env qwen3.8-27b-mtp  # work
spinloop cloud stop --env qwen3.8-27b-mtp    # done — shut it down now
```

The deploy reads the PARAMs the same way `serve` does — `hf` names the
weights, the rest become the server's flags — and drops the settings the
cloud owns (the bind, `ctx-size`, …) from them, exactly as it would from a
preset. See [`spinloop cloud`](../../../docs/commands/cloud.md) for the
rest: `logs`, `metrics`, and how to name and switch between multiple
deployed environments.

## See also

- The same model without MTP, from a `preset.ini`:
  [`examples/llamacpp/qwen3.8-27b`](../qwen3.8-27b/README.md)
- The bigger mixture-of-experts sibling on the same engine:
  [`examples/llamacpp/qwen3.6-35b-a3b`](../qwen3.6-35b-a3b/README.md)
