import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';

// The env Lambda relays base_url/api_key (unaffected by this change) and,
// best-effort, what is deployed to the environment — the same deploy-config
// `start` and `stats` already read, via the same shared helper.

const LAMBDA_ENV = { ENGINE_PORT: '8080' };

const findEnvEip = vi.fn();
const readEnvApiKey = vi.fn();
const readDeployConfig = vi.fn();

vi.mock('../lambda/shared/environments', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/environments')>()),
  findEnvEip: (...args: unknown[]) => findEnvEip(...args),
  readEnvApiKey: (...args: unknown[]) => readEnvApiKey(...args),
}));

vi.mock('../lambda/shared/aws', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/aws')>()),
  readDeployConfig: (...args: unknown[]) => readDeployConfig(...args),
}));

let handler: (event: LambdaFunctionURLEvent) => Promise<LambdaFunctionURLResult>;

beforeAll(async () => {
  Object.assign(process.env, LAMBDA_ENV);
  ({ handler } = await import('../lambda/env/index'));
});

const envEvent = { queryStringParameters: { env: 'dev-3' } } as unknown as LambdaFunctionURLEvent;

function bodyOf(result: LambdaFunctionURLResult): Record<string, unknown> {
  return JSON.parse((result as { statusCode: number; body: string }).body);
}

beforeEach(() => {
  vi.clearAllMocks();
  findEnvEip.mockResolvedValue({ publicIp: '198.51.100.1' });
  readEnvApiKey.mockResolvedValue('sk-remote');
});

describe('env reports what is deployed', () => {
  it('includes the deploy-config fields alongside base_url/api_key', async () => {
    readDeployConfig.mockResolvedValue({
      runner: 'llamacpp',
      modelId: 'org/model',
      servedModelName: 'q3',
      contextSize: 32768,
    });

    const body = bodyOf(await handler(envEvent));
    expect(body).toMatchObject({
      base_url: 'http://198.51.100.1:8080/v1',
      api_key: 'sk-remote',
      deployed: true,
      runner: 'llamacpp',
      modelId: 'org/model',
      servedName: 'q3',
      contextSize: 32768,
    });
  });

  it('omits the deploy-config fields, without failing, when nothing is deployed', async () => {
    readDeployConfig.mockRejectedValue(new Error('deploy-config is not set'));

    const body = bodyOf(await handler(envEvent));
    expect(body).toEqual({ base_url: 'http://198.51.100.1:8080/v1', api_key: 'sk-remote' });
    expect(body).not.toHaveProperty('deployed');
    expect(body).not.toHaveProperty('runner');
  });

  it('omits the deploy-config fields, without failing, when it fails to parse', async () => {
    readDeployConfig.mockRejectedValue(new Error('deploy-config is not valid JSON'));

    const body = bodyOf(await handler(envEvent));
    expect(body).toEqual({ base_url: 'http://198.51.100.1:8080/v1', api_key: 'sk-remote' });
  });
});
