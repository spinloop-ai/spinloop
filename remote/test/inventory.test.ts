import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LambdaFunctionURLEvent } from 'aws-lambda';

// GET {statsUrl} with no `env`: the plane's environment inventory. AWS calls
// are stubbed at the shared-module boundary (the stop.test.ts pattern), so
// these cover the enumeration union, the field assembly and the partial/
// errors accounting — not EC2 or SSM themselves. parseDeployConfig stays real,
// so what a deploy stores is what an inventory reads.

const LAMBDA_ENV = {
  TAG_KEY: 'cloud-vm-llm:managed',
  TAG_VALUE: 'true',
  ENGINE_PORT: '8000',
};

const findManagedInstance = vi.fn();
const findParameter = vi.fn();
const findEnvEip = vi.fn();
const listEnvDeployConfigs = vi.fn();
const taggedEnvNames = vi.fn();
const readDeployConfig = vi.fn();
const runShellCommand = vi.fn();

vi.mock('../lambda/shared/aws', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/aws')>()),
  findManagedInstance: (...args: unknown[]) => findManagedInstance(...args),
  findParameter: (...args: unknown[]) => findParameter(...args),
  readDeployConfig: (...args: unknown[]) => readDeployConfig(...args),
  runShellCommand: (...args: unknown[]) => runShellCommand(...args),
}));

vi.mock('../lambda/shared/environments', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/environments')>()),
  findEnvEip: (...args: unknown[]) => findEnvEip(...args),
  listEnvDeployConfigs: (...args: unknown[]) => listEnvDeployConfigs(...args),
  taggedEnvNames: (...args: unknown[]) => taggedEnvNames(...args),
}));

let handler: (event: LambdaFunctionURLEvent) => Promise<unknown>;

beforeAll(async () => {
  Object.assign(process.env, LAMBDA_ENV);
  ({ handler } = await import('../lambda/stats/index'));
});

function bodyOf(result: unknown): Record<string, any> {
  return JSON.parse((result as { statusCode: number; body: string }).body);
}

function statusOf(result: unknown): number {
  return (result as { statusCode: number }).statusCode;
}

/** GET with no query at all — the inventory call. */
function inventoryEvent(): LambdaFunctionURLEvent {
  return {} as unknown as LambdaFunctionURLEvent;
}

/** A stored deploy-config, ownership stamps included. */
function storedConfig(env: string, over: Record<string, unknown> = {}): string {
  return JSON.stringify({
    runner: 'llamacpp',
    modelId: `org/${env}`,
    quant: 'Q8',
    weightsPrefix: `models/llamacpp/org/${env}/Q8/`,
    contextSize: 131072,
    servedModelName: `${env}-model`,
    serveArgs: [],
    companions: {},
    spinloopVersion: 'latest',
    deployedBy: `arn:aws:sts::0:assumed-role/deployer/${env}`,
    deployedAt: '2026-11-01T09:00:00.000Z',
    ...over,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  listEnvDeployConfigs.mockResolvedValue(new Map<string, string>());
  taggedEnvNames.mockResolvedValue([] as string[]);
  findEnvEip.mockResolvedValue(null);
  findManagedInstance.mockResolvedValue(null);
  findParameter.mockResolvedValue(null);
  runShellCommand.mockResolvedValue({ status: 'Failed', stdout: '' });
});

describe('inventory (GET without env)', () => {
  it('lists every environment with its fields, from configs and tags alike', async () => {
    // dev and lab have stored configs; lab and tagged-only share tags; the
    // tag-only environment has no config at all.
    listEnvDeployConfigs.mockResolvedValue(
      new Map([
        ['dev', storedConfig('dev')],
        ['lab', storedConfig('lab', { runner: 'vllm', deployedBy: 'arn:aws:iam::0:user/other' })],
      ]),
    );
    taggedEnvNames.mockResolvedValue(['lab', 'untagged-state']);
    findEnvEip.mockImplementation((env: string) =>
      Promise.resolve(
        env === 'dev'
          ? { allocationId: 'eipalloc-dev', publicIp: '203.0.113.5' }
          : env === 'untagged-state'
            ? { allocationId: 'eipalloc-odd', publicIp: '203.0.113.9' }
            : null,
      ),
    );
    findManagedInstance.mockImplementation((_key: string, _value: string, filters: unknown[]) =>
      Promise.resolve(
        JSON.stringify(filters).includes('"dev"')
          ? { instanceId: 'i-dev', state: 'running', instanceType: 'g6e.xlarge' }
          : null,
      ),
    );

    const result = await handler(inventoryEvent());
    expect(statusOf(result)).toBe(200);
    const body = bodyOf(result);
    expect(Object.keys(body)).toEqual(['environments']);
    expect(body.environments.map((e: { environment: string }) => e.environment)).toEqual([
      'dev',
      'lab',
      'untagged-state',
    ]);

    const dev = body.environments[0];
    expect(Object.keys(dev).sort()).toEqual(
      [
        'baseUrl',
        'deployedAt',
        'deployedBy',
        'environment',
        'instanceId',
        'instanceType',
        'modelId',
        'partial',
        'runner',
        'servedName',
        'state',
      ].sort(),
    );
    expect(dev).toMatchObject({
      environment: 'dev',
      deployedBy: 'arn:aws:sts::0:assumed-role/deployer/dev',
      deployedAt: '2026-11-01T09:00:00.000Z',
      runner: 'llamacpp',
      modelId: 'org/dev',
      servedName: 'dev-model',
      baseUrl: 'http://203.0.113.5:8000/v1',
      state: 'running',
      instanceType: 'g6e.xlarge',
      instanceId: 'i-dev',
      partial: false,
    });

    // A config with no instance is still listed — stopped, no address.
    expect(body.environments[1]).toMatchObject({
      environment: 'lab',
      runner: 'vllm',
      deployedBy: 'arn:aws:iam::0:user/other',
      state: 'stopped',
      baseUrl: '',
      instanceId: '',
      partial: false,
    });

    // Named only by a tag: everything the config would say is empty, and an
    // absent config is not a failure.
    expect(body.environments[2]).toMatchObject({
      environment: 'untagged-state',
      deployedBy: '',
      deployedAt: '',
      runner: '',
      modelId: '',
      servedName: '',
      baseUrl: 'http://203.0.113.9:8000/v1',
      state: 'stopped',
      instanceType: '',
      instanceId: '',
      partial: false,
    });
    // Named environments query their config only when the listing did not carry it.
    expect(findParameter).toHaveBeenCalledTimes(1);
    expect(findParameter).toHaveBeenCalledWith('/cloud-vm-llm/untagged-state/deploy-config');
  });

  it('answers an empty plane with an empty list at 200', async () => {
    const result = await handler(inventoryEvent());
    expect(statusOf(result)).toBe(200);
    expect(bodyOf(result)).toEqual({ environments: [] });
  });

  it('a failing per-environment read marks that entry partial, notes it, and never fails the listing', async () => {
    listEnvDeployConfigs.mockResolvedValue(
      new Map([
        ['bad', storedConfig('bad')],
        ['good', storedConfig('good')],
      ]),
    );
    findEnvEip.mockImplementation((env: string) =>
      env === 'bad'
        ? Promise.reject(Object.assign(new Error('denied'), { name: 'AccessDeniedException' }))
        : Promise.resolve(null),
    );
    findManagedInstance.mockResolvedValue({ instanceId: 'i-good', state: 'running' });

    const result = await handler(inventoryEvent());
    expect(statusOf(result)).toBe(200);
    const body = bodyOf(result);
    expect(body.environments).toHaveLength(2);
    const [bad, good] = body.environments;
    expect(bad.environment).toBe('bad');
    expect(bad.partial).toBe(true);
    // What could be read is still reported.
    expect(bad.runner).toBe('llamacpp');
    expect(body.errors).toHaveLength(1);
    expect(body.errors[0]).toMatch(/^bad: elastic IP AccessDeniedException$/);
    expect(good.partial).toBe(false);
  });

  it('an unreadable deploy-config marks the entry partial and the rest still list', async () => {
    // The listing failed wholesale, so each environment's own GetParameter is
    // asked — and one of those fails.
    listEnvDeployConfigs.mockResolvedValue(new Map());
    taggedEnvNames.mockResolvedValue(['unreadable', 'never-deployed']);
    findParameter.mockImplementation((name: string) =>
      name.includes('unreadable')
        ? Promise.reject(Object.assign(new Error('denied'), { name: 'AccessDeniedException' }))
        : Promise.resolve(null),
    );

    const body = bodyOf(await handler(inventoryEvent()));
    // Sorted by name: never-deployed first, unreadable second.
    const [never, unreadable] = body.environments;
    expect(unreadable.environment).toBe('unreadable');
    expect(unreadable.partial).toBe(true);
    expect(body.errors.some((e: string) => e.startsWith('unreadable: deploy-config'))).toBe(true);
    // A parameter that is simply absent was never deployed — not partial.
    expect(never.environment).toBe('never-deployed');
    expect(never.partial).toBe(false);
  });

  it('a failing enumeration names the failed source in errors and lists what it can', async () => {
    listEnvDeployConfigs.mockRejectedValue(
      Object.assign(new Error('throttled'), { name: 'ThrottlingException' }),
    );
    taggedEnvNames.mockResolvedValue(['tagged']);
    const body = bodyOf(await handler(inventoryEvent()));
    expect(body.errors).toEqual(['deploy-configs: ThrottlingException']);
    expect(body.environments.map((e: { environment: string }) => e.environment)).toEqual(['tagged']);
  });
});

describe('per-environment stats is unchanged by the branch', () => {
  it('an event with env never reaches the inventory', async () => {
    readDeployConfig.mockResolvedValue({ runner: 'llamacpp', modelId: 'org/m' });
    findManagedInstance.mockResolvedValue({ instanceId: 'i-run', state: 'running' });
    const event = { queryStringParameters: { env: 'dev' } } as unknown as LambdaFunctionURLEvent;
    const result = await handler(event);
    expect(statusOf(result)).toBe(200);
    expect(bodyOf(result).state).toBe('running');
    expect(listEnvDeployConfigs).not.toHaveBeenCalled();
    expect(taggedEnvNames).not.toHaveBeenCalled();
  });

  it('an empty env value is still the old 400, not an inventory', async () => {
    const event = { queryStringParameters: { env: '' } } as unknown as LambdaFunctionURLEvent;
    const result = await handler(event);
    expect(statusOf(result)).toBe(400);
    expect(listEnvDeployConfigs).not.toHaveBeenCalled();
  });
});
