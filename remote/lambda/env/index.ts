/**
 * Env Lambda — returns the API key and base URL for an environment, plus what
 * is currently deployed to it, if anything.
 * Does NOT start the instance: the API key lives in Secrets Manager, the EIP
 * is allocated at deploy, and the deploy-config lives in SSM, so all three are
 * available regardless of instance state.
 *
 * The caller (spinloop harness) uses base_url/api_key to inject
 * OPENAI_API_KEY and OPENAI_BASE_URL into the agent's environment, so the
 * user never has to export anything manually. With no Spinloop applied, it
 * also uses the deploy-config fields (when present) to configure the harness
 * itself — the provider, model and context window — from what is actually
 * running, rather than requiring a local Spinloop to restate it.
 */

import type { LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';
import {
  errorName,
  readDeployConfig,
  requireEnv,
} from '../shared/aws';
import {
  baseUrlFor,
  deployConfigParam,
  environmentFrom,
  findEnvEip,
  readEnvApiKey,
} from '../shared/environments';
import { jsonResponse } from '../shared/http';

const ENGINE_PORT = requireEnv('ENGINE_PORT');

/**
 * Read the environment's deploy-config for the facts that name what it is
 * serving, the same facts `start` and `stats` already read from the same
 * source. Absent or unparsable degrades to an empty object rather than
 * failing the call: base_url/api_key are still useful on their own for an
 * environment with nothing deployed yet, or one deployed before this field
 * existed.
 */
async function readDeployFacts(env: string): Promise<{
  deployed?: true;
  runner?: string;
  modelId?: string;
  servedName?: string;
  contextSize?: number;
}> {
  try {
    const cfg = await readDeployConfig(deployConfigParam(env));
    return {
      deployed: true,
      runner: cfg.runner,
      modelId: cfg.modelId,
      servedName: cfg.servedModelName,
      contextSize: cfg.contextSize,
    };
  } catch (err) {
    console.log(JSON.stringify({ phase: 'deploy-facts', environment: env, error: errorName(err) }));
    return {};
  }
}

export async function handler(event: LambdaFunctionURLEvent): Promise<LambdaFunctionURLResult> {
  let env: string;
  try {
    env = environmentFrom(event.queryStringParameters);
  } catch (err) {
    return jsonResponse(400, { error: (err as Error).message });
  }

  try {
    const eip = await findEnvEip(env);
    if (!eip) {
      return jsonResponse(500, {
        error: 'environment has no Elastic IP allocated',
      });
    }

    const apiKey = await readEnvApiKey(env);
    if (!apiKey) {
      return jsonResponse(500, {
        error: 'environment has no API key',
      });
    }

    const baseURL = baseUrlFor(eip.publicIp, ENGINE_PORT);
    const deployFacts = await readDeployFacts(env);

    return jsonResponse(200, {
      base_url: baseURL,
      api_key: apiKey,
      ...deployFacts,
    });
  } catch (err) {
    console.log(JSON.stringify({ environment: env, error: errorName(err) }));
    return jsonResponse(500, {
      error: `failed to retrieve endpoint environment: ${errorName(err)}`,
    });
  }
}
