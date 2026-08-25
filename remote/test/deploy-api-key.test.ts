import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LambdaFunctionURLEvent } from 'aws-lambda';

// Controlled per test: what the secret's state makes the action, and what the
// Lambda handed it.
const keyCalls: (string | undefined)[] = [];
let keyAction = 'rotated';
const persisted: unknown[] = [];

vi.mock('../lambda/shared/aws', () => ({
  requireEnv: (name: string) => (name === 'ENGINE_PORT' ? '8000' : `stub-${name}`),
  readDeployConfig: vi.fn(),
  writeDeployConfig: vi.fn(async (_param: string, cfg: unknown) => {
    persisted.push(cfg);
  }),
  errorName: (err: unknown) => (err instanceof Error ? err.name : String(err)),
}));

vi.mock('../lambda/shared/environments', () => ({
  environmentFrom: (_q: unknown, body: unknown) => (body as string) ?? 'default',
  deployConfigParam: (env: string) => `/cloud-vm-llm/${env}/deploy-config`,
  baseUrlFor: (ip: string, port: number) => `http://${ip}:${port}/v1`,
  ensureEnvEip: async () => ({ publicIp: '198.51.100.7' }),
  ensureEnvSecurityGroup: async () => undefined,
  findEnvSecurityGroup: async () => 'sg-1',
  ensureEnvApiKey: async (_env: string, key?: string) => {
    keyCalls.push(key);
    return keyAction;
  },
}));

vi.mock('../lambda/shared/seed', () => ({
  weightsPresent: async () => true,
}));

vi.mock('../lambda/shared/seed/launch', () => ({
  seedInfraFromEnv: () => ({ bucket: 'weights' }),
  buildSeedJob: (cfg: unknown) => ({ seedId: 'llamacpp--m', cfg }),
  launchSeedInstance: async (job: { seedId: string }, _infra: unknown, _opts?: { force?: boolean }) => {
    return { seedId: job.seedId, instanceId: 'i-seed', started: true };
  },
}));

let handler: (event: LambdaFunctionURLEvent) => Promise<{ statusCode: number; body: string }>;

beforeAll(async () => {
  ({ handler } = (await import('../lambda/deploy/index')) as never);
});

const CONFIG = {
  environment: 'glimmer',
  runner: 'llamacpp',
  modelId: 'meta-models/Muse-Glimmer-30B-GGUF',
  quant: 'kquant-dynamic',
  contextSize: 524288,
  servedModelName: 'muse-glimmer-30b',
  serveArgs: [],
  allowedCidr: '203.0.113.7/32',
};

function post(body: Record<string, unknown>): LambdaFunctionURLEvent {
  return {
    requestContext: { http: { method: 'POST' } },
    body: JSON.stringify(body),
    isBase64Encoded: false,
  } as unknown as LambdaFunctionURLEvent;
}

beforeEach(() => {
  keyCalls.length = 0;
  persisted.length = 0;
  keyAction = 'rotated';
});

describe('deploy with a supplied key', () => {
  it('stores the key in the environment\'s secret and reports the action', async () => {
    const res = await handler(post({ ...CONFIG, apiKey: 'sk-supplied' }));
    expect(res.statusCode).toBe(200);
    expect(keyCalls).toEqual(['sk-supplied']);
    const reply = JSON.parse(res.body);
    expect(reply.apiKeyAction).toBe('rotated');
    // The action, never the value.
    expect(res.body).not.toContain('sk-supplied');
  });

  it('reports a created key the same way', async () => {
    keyAction = 'created';
    const res = await handler(post({ ...CONFIG, apiKey: 'sk-supplied' }));
    expect(res.statusCode).toBe(200);
    expect(JSON.parse(res.body).apiKeyAction).toBe('created');
  });

  it('never persists the key into the stored deploy-config', async () => {
    // Stored, it would be re-sent on every wake that read the config back.
    await handler(post({ ...CONFIG, apiKey: 'sk-supplied' }));
    expect(persisted).toHaveLength(1);
    expect(persisted[0]).not.toHaveProperty('apiKey');
  });

  it('leaves the secret alone when no key is sent', async () => {
    const res = await handler(post(CONFIG));
    expect(res.statusCode).toBe(200);
    expect(keyCalls).toEqual([undefined]);
    expect(JSON.parse(res.body)).not.toHaveProperty('apiKeyAction');
  });

  it('rejects a non-string apiKey', async () => {
    const res = await handler(post({ ...CONFIG, apiKey: 42 }));
    expect(res.statusCode).toBe(400);
    expect(JSON.parse(res.body).error).toMatch(/apiKey must be a string/);
  });
});
