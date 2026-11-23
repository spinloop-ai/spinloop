import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Context, LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';
import { schedulerNamePrefix } from '../lambda/shared/schedules';

// `POST {stopUrl}?action=undeploy&env=<name>`: taking exactly one environment
// apart. Every AWS SDK client is stubbed at the client boundary (the
// env-api-key.test.ts pattern) with the real command classes kept, so the
// assertions inspect the actual command inputs — every filter, name and id
// must carry this one environment.

const LAMBDA_ENV = {
  TAG_KEY: 'cloud-vm-llm:managed',
  TAG_VALUE: 'true',
  IDLE_THRESHOLD_MINUTES: '15',
  GRACE_PERIOD_MINUTES: '10',
  MAX_RUNTIME_MINUTES: '240',
  STOP_RETENTION_MINUTES: '60',
  MAX_SEED_MINUTES: '60',
  SEED_STALL_MINUTES: '10',
  SCHEDULE_GROUP: 'test-group',
};

const ec2Send = vi.fn();
const ssmSend = vi.fn();
const secretsSend = vi.fn();
const schedulerSend = vi.fn();

/** Stub the client but keep the real command classes, so `input` is inspectable. */
function stubClient(client: string, send: (cmd: unknown) => Promise<unknown>) {
  return async (importOriginal: () => Promise<Record<string, unknown>>) => ({
    ...(await importOriginal()),
    [client]: vi.fn().mockImplementation(() => ({ send: (cmd: unknown) => send(cmd) })),
  });
}

vi.mock('@aws-sdk/client-ec2', stubClient('EC2Client', (cmd) => ec2Send(cmd)) as never);
vi.mock('@aws-sdk/client-ssm', stubClient('SSMClient', (cmd) => ssmSend(cmd)) as never);
vi.mock(
  '@aws-sdk/client-secrets-manager',
  stubClient('SecretsManagerClient', (cmd) => secretsSend(cmd)) as never,
);
vi.mock(
  '@aws-sdk/client-scheduler',
  stubClient('SchedulerClient', (cmd) => schedulerSend(cmd)) as never,
);

const awsError = (name: string) => Object.assign(new Error(name), { name });
const commandName = (cmd: { constructor: { name: string } }) => cmd.constructor.name;

/** What a recorder saw, as [command name, input]. */
function sent(rec: ReturnType<typeof vi.fn>): Array<[string, Record<string, any>]> {
  return rec.mock.calls.map(([cmd]) => [commandName(cmd), cmd.input]);
}
const sentNames = (rec: ReturnType<typeof vi.fn>) => sent(rec).map(([name]) => name);

let handler: (event: LambdaFunctionURLEvent, context?: Context) => Promise<LambdaFunctionURLResult | void>;

beforeEach(async () => {
  vi.clearAllMocks();
  Object.assign(process.env, LAMBDA_ENV);
  vi.resetModules();
  ({ handler } = await import('../lambda/stop/index'));
});

function undeployEvent(dryRun = false): LambdaFunctionURLEvent {
  const query: Record<string, string> = { env: 'glimmer', action: 'undeploy' };
  if (dryRun) {
    query.dryRun = 'true';
  }
  return {
    queryStringParameters: query,
    requestContext: { http: { method: 'POST' } },
  } as unknown as LambdaFunctionURLEvent;
}

function bodyOf(result: LambdaFunctionURLResult | void): Record<string, any> {
  return JSON.parse((result as { statusCode: number; body: string }).body);
}

/** An empty account: nothing tagged, no parameters, no secret, no schedules. */
function emptyPlane() {
  ec2Send.mockImplementation(async (cmd: unknown) => {
    switch (commandName(cmd as never)) {
      case 'DescribeInstancesCommand':
        return { Reservations: [] };
      case 'DescribeAddressesCommand':
        return { Addresses: [] };
      case 'DescribeSecurityGroupsCommand':
        return { SecurityGroups: [] };
      default:
        return {};
    }
  });
  ssmSend.mockImplementation(async (cmd: unknown) => {
    switch (commandName(cmd as never)) {
      case 'GetParameterCommand':
        throw awsError('ParameterNotFound');
      case 'DeleteParameterCommand':
        throw awsError('ParameterNotFound');
      default:
        return {};
    }
  });
  secretsSend.mockImplementation(async (cmd: unknown) => {
    if (commandName(cmd as never) === 'DescribeSecretCommand') {
      throw awsError('ResourceNotFoundException');
    }
    return {};
  });
  schedulerSend.mockImplementation(async (cmd: unknown) =>
    commandName(cmd as never) === 'ListSchedulesCommand' ? { Schedules: [] } : {},
  );
}

/** The whole environment present: instance, EIP, SG, secret, schedules, config, wake-lock — plus a second environment nothing here may touch. */
function fullPlane() {
  emptyPlane();
  ec2Send.mockImplementation(async (cmd: unknown) => {
    switch (commandName(cmd as never)) {
      case 'DescribeInstancesCommand':
        return {
          Reservations: [
            {
              Instances: [
                {
                  InstanceId: 'i-run',
                  State: { Name: 'running' },
                  Tags: [
                    { Key: 'cloud-vm-llm:managed', Value: 'true' },
                    { Key: 'cloud-vm-llm:env', Value: 'glimmer' },
                  ],
                },
              ],
            },
          ],
        };
      case 'DescribeAddressesCommand':
        return {
          Addresses: [{ AllocationId: 'eipalloc-1', PublicIp: '203.0.113.7' }],
        };
      case 'DescribeSecurityGroupsCommand':
        return { SecurityGroups: [{ GroupId: 'sg-1' }] };
      default:
        return {};
    }
  });
  ssmSend.mockImplementation(async (cmd: { input: { Name?: string } }) => {
    if (commandName(cmd) === 'GetParameterCommand') {
      if (cmd.input.Name === '/cloud-vm-llm/glimmer/deploy-config') {
        return { Parameter: { Value: '{"runner":"llamacpp","modelId":"org/m","servedModelName":"m","contextSize":1}' } };
      }
      if (cmd.input.Name === '/cloud-vm-llm/glimmer/schedules') {
        return { Parameter: { Value: '[]' } };
      }
      throw awsError('ParameterNotFound');
    }
    if (commandName(cmd) === 'DeleteParameterCommand') {
      // wake-lock was never there; the other two were.
      if (cmd.input.Name === '/cloud-vm-llm/glimmer/wake-lock') {
        throw awsError('ParameterNotFound');
      }
      return {};
    }
    return {};
  });
  secretsSend.mockImplementation(async (cmd: unknown) =>
    commandName(cmd as never) === 'DescribeSecretCommand' ? {} : {},
  );
  schedulerSend.mockImplementation(async () => ({
    Schedules: [{ Name: `${schedulerNamePrefix('glimmer')}1` }],
  }));
}

describe('undeploy', () => {
  it('releases every resource of the named environment and lists exactly those', async () => {
    fullPlane();

    const result = await handler(undeployEvent(), {} as Context);
    const body = bodyOf(result);
    expect((result as { statusCode: number }).statusCode).toBe(200);
    expect(body).toEqual({
      environment: 'glimmer',
      released: [
        'instance:i-run',
        'elastic-ip:eipalloc-1',
        'security-group:sg-1',
        'api-key',
        'schedules-parameter',
        `schedule:${schedulerNamePrefix('glimmer')}1`,
        'deploy-config',
      ],
    });

    // Instance discovery is scoped to the plane tag AND this one environment.
    const [describe] = sent(ec2Send);
    expect(describe[0]).toBe('DescribeInstancesCommand');
    expect(describe[1].Filters).toEqual(
      expect.arrayContaining([
        { Name: 'tag:cloud-vm-llm:managed', Values: ['true'] },
        { Name: 'tag:cloud-vm-llm:env', Values: ['glimmer'] },
      ]),
    );
    // EIP and SG are found by the env tag alone.
    for (const [name, input] of sent(ec2Send)) {
      if (name === 'DescribeAddressesCommand' || name === 'DescribeSecurityGroupsCommand') {
        expect(input.Filters).toEqual([{ Name: 'tag:cloud-vm-llm:env', Values: ['glimmer'] }]);
      }
    }
    expect(sent(ec2Send)).toContainEqual(['TerminateInstancesCommand', { InstanceIds: ['i-run'] }]);
    expect(sent(ec2Send)).toContainEqual(['ReleaseAddressCommand', { AllocationId: 'eipalloc-1' }]);
    expect(sent(ec2Send)).toContainEqual(['DeleteSecurityGroupCommand', { GroupId: 'sg-1' }]);
    // The group is deleted only after the instance is gone.
    expect(sentNames(ec2Send).indexOf('DeleteSecurityGroupCommand')).toBeGreaterThan(
      sentNames(ec2Send).indexOf('TerminateInstancesCommand'),
    );
    // Secret deleted for this environment's key only, past recovery.
    expect(sent(secretsSend)).toContainEqual([
      'DeleteSecretCommand',
      { SecretId: 'cloud-vm-llm/glimmer/api-key', ForceDeleteWithoutRecovery: true },
    ]);
    // Only this environment's SSM state was deleted — and its schedule was
    // deleted from the group by its own hashed-name prefix. The wake-lock was
    // never there; its tolerated ParameterNotFound does not make the report.
    for (const [, input] of sent(ssmSend).filter(([n]) => n === 'DeleteParameterCommand')) {
      expect(input.Name.startsWith('/cloud-vm-llm/glimmer/')).toBe(true);
    }
    expect(sentNames(ssmSend).filter((n) => n === 'DeleteParameterCommand')).toHaveLength(3);
    expect(sent(schedulerSend)).toContainEqual([
      'DeleteScheduleCommand',
      { Name: `${schedulerNamePrefix('glimmer')}1`, GroupName: 'test-group' },
    ]);
    expect(schedulerNamePrefix('glimmer')).not.toBe(schedulerNamePrefix('other'));
  });

  it('refuses an unknown environment with a 404 and deletes nothing', async () => {
    emptyPlane();

    const result = await handler(undeployEvent(), {} as Context);
    expect((result as { statusCode: number }).statusCode).toBe(404);
    expect(bodyOf(result).error).toMatch(/unknown environment "glimmer"/);
    expect(sentNames(ec2Send).every((n) => n.startsWith('Describe'))).toBe(true);
    expect(sentNames(ssmSend)).not.toContain('DeleteParameterCommand');
    expect(sentNames(secretsSend)).not.toContain('DeleteSecretCommand');
    expect(sentNames(schedulerSend)).not.toContain('DeleteScheduleCommand');
  });

  it('reports what it would release on dryRun and deletes nothing', async () => {
    fullPlane();
    // wake-lock present for the dry run — the read is what reports it.
    ssmSend.mockImplementation(async (cmd: { input: { Name?: string } }) => {
      if (commandName(cmd) === 'GetParameterCommand') {
        if (cmd.input.Name?.includes('wake-lock')) {
          return { Parameter: { Value: '{"owner":"i-x","expiresAt":"2030-01-01T00:00:00.000Z"}' } };
        }
        if (cmd.input.Name === '/cloud-vm-llm/glimmer/deploy-config') {
          return { Parameter: { Value: '{"runner":"llamacpp","modelId":"org/m","servedModelName":"m","contextSize":1}' } };
        }
        if (cmd.input.Name === '/cloud-vm-llm/glimmer/schedules') {
          return { Parameter: { Value: '[]' } };
        }
      }
      throw awsError('ParameterNotFound');
    });

    const result = await handler(undeployEvent(true), {} as Context);
    expect((result as { statusCode: number }).statusCode).toBe(200);
    const body = bodyOf(result);
    expect(body.dryRun).toBe(true);
    expect(body.released).toEqual([
      'instance:i-run',
      'elastic-ip:eipalloc-1',
      'security-group:sg-1',
      'api-key',
      'schedules-parameter',
      `schedule:${schedulerNamePrefix('glimmer')}1`,
      'deploy-config',
      'wake-lock',
    ]);
    // Nothing mutating was sent, to any service.
    const mutating = ['TerminateInstancesCommand', 'ReleaseAddressCommand', 'DeleteSecurityGroupCommand'];
    expect(sentNames(ec2Send).some((n) => mutating.includes(n))).toBe(false);
    expect(sentNames(ssmSend).some((n) => n.startsWith('Delete'))).toBe(false);
    expect(sentNames(secretsSend).some((n) => n.startsWith('Delete'))).toBe(false);
    expect(sentNames(schedulerSend).some((n) => n.startsWith('Delete'))).toBe(false);
  });

  it('names only what was actually there when resources are already gone', async () => {
    emptyPlane();
    // Only the deploy-config remains — instance, EIP, SG, secret, schedules
    // and wake-lock were never there (or went first).
    ssmSend.mockImplementation(async (cmd: { input: { Name?: string } }) => {
      if (commandName(cmd) === 'GetParameterCommand' && cmd.input.Name === '/cloud-vm-llm/glimmer/deploy-config') {
        return { Parameter: { Value: '{"runner":"llamacpp","modelId":"org/m","servedModelName":"m","contextSize":1}' } };
      }
      if (commandName(cmd) === 'DeleteParameterCommand' && cmd.input.Name === '/cloud-vm-llm/glimmer/deploy-config') {
        return {};
      }
      throw awsError('ParameterNotFound');
    });

    const body = bodyOf(await handler(undeployEvent(), {} as Context));
    expect(body.released).toEqual(['deploy-config']);
    // An absent secret and absent schedules are tolerated, not errors.
    expect(secretsSend.mock.calls.length).toBeGreaterThan(0); // described, not deleted
    expect(sentNames(secretsSend)).not.toContain('DeleteSecretCommand');
  });

  it('a GET with the undeploy action is still the state report, never a teardown', async () => {
    emptyPlane();
    const event = {
      ...undeployEvent(),
      requestContext: { http: { method: 'GET' } },
    } as unknown as LambdaFunctionURLEvent;
    const body = bodyOf(await handler(event, {} as Context));
    expect(body.state).toBe('stopped');
    expect(sentNames(ec2Send).every((n) => n.startsWith('Describe'))).toBe(true);
  });
});

describe('deleteSecurityGroup dependency tolerance', () => {
  it('rides out a brief DependencyViolation from the just-terminated instance', async () => {
    emptyPlane();
    ec2Send.mockImplementation(async (cmd: unknown) => {
      if (commandName(cmd as never) === 'DeleteSecurityGroupCommand') {
        if (ec2Send.mock.calls.length === 1) {
          throw awsError('DependencyViolation');
        }
        return {};
      }
      return {};
    });
    const { deleteSecurityGroup } = await import('../lambda/shared/environments');
    await expect(deleteSecurityGroup('sg-1', 1)).resolves.toBeUndefined();
    expect(sentNames(ec2Send).filter((n) => n === 'DeleteSecurityGroupCommand')).toHaveLength(2);
  });

  it('surfaces any other delete failure', async () => {
    emptyPlane();
    ec2Send.mockImplementation(async (cmd: unknown) => {
      if (commandName(cmd as never) === 'DeleteSecurityGroupCommand') {
        throw awsError('InvalidGroup.NotFound');
      }
      return {};
    });
    const { deleteSecurityGroup } = await import('../lambda/shared/environments');
    await expect(deleteSecurityGroup('sg-1', 1)).rejects.toThrow('InvalidGroup.NotFound');
  });
});

describe('plane enumeration (shared environments helpers)', () => {
  it('keys every deploy-config under the plane path by environment, across pages', async () => {
    emptyPlane();
    let pages = 0;
    ssmSend.mockImplementation(async (cmd: unknown) => {
      if (commandName(cmd as never) !== 'GetParametersByPathCommand') {
        return {};
      }
      pages += 1;
      return pages === 1
        ? {
            Parameters: [{ Name: '/cloud-vm-llm/dev/deploy-config', Value: '{"runner":"llamacpp"}' }],
            NextToken: 't',
          }
        : {
            Parameters: [
              // Not a deploy-config, or not this plane's — neither enumerates.
              { Name: '/cloud-vm-llm/dev/schedules', Value: '[]' },
              { Name: '/other-plane/lab/deploy-config', Value: '{}' },
              { Name: '/cloud-vm-llm/lab/deploy-config', Value: '{"runner":"vllm"}' },
            ],
          };
    });
    const { listEnvDeployConfigs } = await import('../lambda/shared/environments');
    const configs = await listEnvDeployConfigs();
    expect([...configs.keys()].sort()).toEqual(['dev', 'lab']);
    const listings = sent(ssmSend).filter(([n]) => n === 'GetParametersByPathCommand');
    expect(listings).toHaveLength(2);
    expect(listings[0][1]).toMatchObject({ Path: '/cloud-vm-llm/', Recursive: true });
  });

  it('names every environment the env tag mentions — EIPs, groups and instances alike', async () => {
    emptyPlane();
    ec2Send.mockImplementation(async (cmd: unknown) => {
      const filters = (cmd as { input: { Filters?: unknown } }).input.Filters;
      // A `*` tag-value filter: everything this plane names, scoped by tag.
      expect(filters).toEqual([{ Name: 'tag:cloud-vm-llm:env', Values: ['*'] }]);
      switch (commandName(cmd as never)) {
        case 'DescribeAddressesCommand':
          return { Addresses: [{ Tags: [{ Key: 'cloud-vm-llm:env', Value: 'dev' }] }] };
        case 'DescribeSecurityGroupsCommand':
          return {
            SecurityGroups: [
              { Tags: [{ Key: 'cloud-vm-llm:env', Value: 'lab' }] },
              { Tags: [{ Key: 'Name', Value: 'untagged-for-us' }] },
            ],
          };
        case 'DescribeInstancesCommand':
          return {
            Reservations: [
              {
                Instances: [
                  { Tags: [{ Key: 'cloud-vm-llm:env', Value: 'dev' }] },
                  { Tags: [{ Key: 'cloud-vm-llm:env', Value: 'solo' }] },
                ],
              },
            ],
          };
        default:
          return {};
      }
    });
    const { taggedEnvNames } = await import('../lambda/shared/environments');
    expect((await taggedEnvNames()).sort()).toEqual(['dev', 'lab', 'solo']);
  });
});
