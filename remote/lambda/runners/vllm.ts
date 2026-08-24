/**
 * vLLM's runner spec: whole-checkpoint weights served from the model
 * directory, and the API key delivered by env file (wired into the daemon unit
 * via EnvironmentFile).
 */

import type { DeployConfig } from '../shared/deploy-config';
import { daemonBoot, daemonDeployConfig } from './daemon-boot';
import type { RunnerSpec } from './spec';

const syncedModelPath = (modelDir: string): string => modelDir;

const daemonDeployConfigJson = (
  cfg: DeployConfig,
  modelDir: string,
  port: number,
): string => daemonDeployConfig(cfg, syncedModelPath(modelDir), port, [
    '--gpu-memory-utilization',
    '0.92',
    // Triton attention, not FlashInfer's: FlashInfer's sampler workaround
    // above only covers sampling — its *attention* kernels are JIT-compiled
    // on the first prefill, which needs nvcc and kills the engine on the
    // first real request. The slim AMI ships no toolkit and vllm's wheel
    // pulls flashinfer-python but not flash-attn, so Triton is the one
    // attention backend that runs: its kernels compile at engine init
    // (torch.compile), never at request time. A preset's own
    // --attention-backend still wins — the preset's args follow the
    // runner's. This flag covers the target model only: a speculative
    // drafter never inherits it (vLLM autoselects for the drafter, and
    // its autoselect lands on FlashInfer here), so a preset that enables
    // speculative decoding must also carry "attention_backend":
    // "TRITON_ATTN" inside its speculative-config JSON.
    '--attention-backend',
    'TRITON_ATTN',
  ]);

export const vllm: RunnerSpec = {
  // vLLM serves the whole synced checkpoint directory.
  syncedModelPath,

  // The whole checkpoint. vLLM loads config, tokeniser and weights from the
  // directory, so there is nothing to filter out.
  seedSelection: () => ({ include: ['*'] }),

  // vLLM serves a whole checkpoint and has no companion-file flags of its own,
  // so a companion named for a vLLM deployment is seeded but unused rather
  // than rejected — see the companionArgs contract in spec.ts.
  companionArgs: () => [],

  daemonDeployConfigJson,

  daemonBoot: (cfg, modelDir, port) => `# Python dev headers: Triton JIT-compiles a CUDA stub against Python.h on the
# first model load (Qwen3.6's linear-attention path); baked into recipe 2.0.3+,
# this is a safety net for instances off an older AMI and a no-op once present.
if [ ! -f /usr/include/python3.12/Python.h ]; then
  apt-get update && apt-get install -y python3.12-dev
fi

cat >/etc/vllm.env <<ENVFILE
VLLM_API_KEY=$API_KEY
HF_HUB_OFFLINE=1
# Native Torch sampler, not FlashInfer's — FlashInfer JIT-needs nvcc, which the
# slim AMI (driver + CUDA runtime only, no toolkit) does not ship.
VLLM_USE_FLASHINFER_SAMPLER=0
ENVFILE

${daemonBoot(daemonDeployConfigJson(cfg, modelDir, port), 'EnvironmentFile=/etc/vllm.env\n')}`,
};
