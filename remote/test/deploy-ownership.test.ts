import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LambdaFunctionURLEvent } from 'aws-lambda';

// Who deployed what, and when: the deploy Lambda stamps the stored config and
// its reply with the authenticated identity its request arrived under — never
// with anything the body claimed — so `spinloop cloud deploy` leaves an audit
// trail and a redeploy replaces it.

const persisted: Record<string, any>[] = [];

vi.mock('../lambda/shared/aws', () => ({
  requireEnv: (name: string) => (name === 'ENGINE_PORT' ? '8000' : `stub-${name}`),
  readDeployConfig: vi.fn(),
  writeDeployConfig: vi.fn(async (_param: string, cfg: unknown) => {
    persisted.push(cfg as Record<string, any>);
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
  ensureEnvApiKey: async () => 'unchanged',
}));

vi.mock('../lambda/shared/seed', () => ({
  weightsPresent: async () => true,
}));

vi.mock('../lambda/shared/seed/launch', () => ({
  seedInfraFromEnv: () => ({ bucket: 'weights' }),
  buildSeedJob: (cfg: unknown) => ({ seedId: 'llamacpp--m', cfg }),
  launchSeedInstance: async () => ({ seedId: 's', instanceId: 'i-seed', started: true }),
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

/** A POST under the given request context — the authenticated shape varies, the stamp must not fail. */
function post(body: Record<string, unknown>, requestContext: Record<string, unknown> = {}): LambdaFunctionURLEvent {
  return {
    requestContext: { http: { method: 'POST' }, ...requestContext },
    body: JSON.stringify(body),
    isBase64Encoded: false,
  } as unknown as LambdaFunctionURLEvent;
}

const IAM = {
  authorizer: { iam: { userArn: 'arn:aws:sts::0:assumed-role/admin/pete', accessKey: 'ASIA1' } },
};

beforeEach(() => {
  persisted.length = 0;
});

describe('deploy ownership', () => {
  it('records the invoking principal and the time, stored and echoed back', async () => {
    const res = await handler(post(CONFIG, IAM));
    expect(res.statusCode).toBe(200);
    const reply = JSON.parse(res.body);
    expect(persisted).toHaveLength(1);
    expect(persisted[0].deployedBy).toBe('arn:aws:sts::0:assumed-role/admin/pete');
    expect(persisted[0].deployedAt).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
    // The reply carries both back, so the caller can trust what was written.
    expect(reply.deployedBy).toBe(persisted[0].deployedBy);
    expect(reply.deployedAt).toBe(persisted[0].deployedAt);
  });

  it('a redeploy replaces both stamps with the new caller and time', async () => {
    await handler(post(CONFIG, IAM));
    await handler(
      post(CONFIG, { authorizer: { iam: { userArn: 'arn:aws:iam::0:user/other' } } }),
    );
    expect(persisted).toHaveLength(2);
    expect(persisted[1].deployedBy).toBe('arn:aws:iam::0:user/other');
    // A re-run within the same second could share the stamp's second; the
    // replacement is structural, so just require a fresh valid stamp.
    expect(persisted[1].deployedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it('reads through the identity request-context shape too', async () => {
    await handler(post(CONFIG, { identity: { sourceArn: 'arn:aws:sts::0:assumed-role/ops/sam' } }));
    expect(persisted[0].deployedBy).toBe('arn:aws:sts::0:assumed-role/ops/sam');
  });

  it('falls back to unknown rather than failing a deploy with no identity metadata', async () => {
    const res = await handler(post(CONFIG));
    expect(res.statusCode).toBe(200);
    expect(persisted[0].deployedBy).toBe('unknown');
    // Still stamped with a time — the when is always known, whoever the who is.
    expect(persisted[0].deployedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it('overwrites any deployedBy the body claims — the stamp is identity, never input', async () => {
    // Same guard the weightsPrefix and apiKey already have: what arrives in
    // the body is not trusted for ownership.
    const res = await handler(post({ ...CONFIG, deployedBy: 'arn:aws:iam::0:user:victim' }, IAM));
    expect(res.statusCode).toBe(200);
    expect(persisted[0].deployedBy).toBe('arn:aws:sts::0:assumed-role/admin/pete');
  });

  it('the stamps are plain metadata: present in the stored document, and no secret sits beside them', async () => {
    await handler(post({ ...CONFIG, apiKey: 'sk-supplied' }, IAM));
    const stored = JSON.stringify(persisted[0]);
    expect(stored).toContain('deployedBy');
    expect(stored).toContain('deployedAt');
    // The key never lands in the document the stamps live in.
    expect(stored).not.toContain('sk-supplied');
  });
});
