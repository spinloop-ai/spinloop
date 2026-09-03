import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  CreateSecretCommand,
  DescribeSecretCommand,
  PutSecretValueCommand,
} from '@aws-sdk/client-secrets-manager';

// Every command the Lambda constructs, for the assertions.
const sent: unknown[] = [];
// The secret's state as DescribeSecretCommand finds it.
let outcome: 'exists' | 'missing' = 'missing';

vi.mock('@aws-sdk/client-ec2', () => ({
  EC2Client: vi.fn().mockImplementation(() => ({})),
  AllocateAddressCommand: class {},
  AuthorizeSecurityGroupIngressCommand: class {},
  CreateSecurityGroupCommand: class {},
  DescribeAddressesCommand: class {},
  DescribeSecurityGroupsCommand: class {},
  RevokeSecurityGroupIngressCommand: class {},
}));

vi.mock('@aws-sdk/client-secrets-manager', () => {
  class CreateSecretCommand {
    input: unknown;
    constructor(input: unknown) {
      this.input = input;
    }
  }
  class DescribeSecretCommand {
    input: unknown;
    constructor(input: unknown) {
      this.input = input;
    }
  }
  class GetSecretValueCommand {
    input: unknown;
    constructor(input: unknown) {
      this.input = input;
    }
  }
  class PutSecretValueCommand {
    input: unknown;
    constructor(input: unknown) {
      this.input = input;
    }
  }
  const send = async (cmd: unknown) => {
    sent.push(cmd);
    if (cmd instanceof DescribeSecretCommand) {
      if (outcome === 'missing') {
        const err = new Error("Secrets Manager can't find the specified secret");
        err.name = 'ResourceNotFoundException';
        throw err;
      }
      return {};
    }
    return {};
  };
  return {
    CreateSecretCommand,
    DescribeSecretCommand,
    GetSecretValueCommand,
    PutSecretValueCommand,
    SecretsManagerClient: vi.fn().mockImplementation(() => ({ send })),
  };
});

import { apiKeySecretName, ensureEnvApiKey } from '../lambda/shared/environments';

describe('ensureEnvApiKey', () => {
  beforeEach(() => {
    sent.length = 0;
    outcome = 'missing';
  });

  it('creates the secret with a supplied key when it is absent', async () => {
    const action = await ensureEnvApiKey('glimmer', 'sk-supplied');
    expect(action).toBe('created');
    const create = sent.find((c) => c instanceof CreateSecretCommand) as CreateSecretCommand;
    expect(create.input).toMatchObject({
      Name: apiKeySecretName('glimmer'),
      SecretString: 'sk-supplied',
    });
    expect(sent.some((c) => c instanceof PutSecretValueCommand)).toBe(false);
  });

  it('rotates an existing secret to the supplied key', async () => {
    outcome = 'exists';
    const action = await ensureEnvApiKey('glimmer', 'sk-new');
    expect(action).toBe('rotated');
    const put = sent.find((c) => c instanceof PutSecretValueCommand) as PutSecretValueCommand;
    expect(put.input).toMatchObject({
      SecretId: apiKeySecretName('glimmer'),
      SecretString: 'sk-new',
    });
    // The old value is replaced in place, not a new secret minted.
    expect(sent.some((c) => c instanceof CreateSecretCommand)).toBe(false);
  });

  it('leaves an existing secret alone without a key', async () => {
    outcome = 'exists';
    const action = await ensureEnvApiKey('glimmer');
    expect(action).toBe('unchanged');
    // Only the describe — no create, no put.
    expect(sent).toHaveLength(1);
    expect(sent[0] instanceof DescribeSecretCommand).toBe(true);
  });

  it('generates a value on first creation when no key is supplied', async () => {
    const action = await ensureEnvApiKey('glimmer');
    expect(action).toBe('created');
    const create = sent.find((c) => c instanceof CreateSecretCommand) as CreateSecretCommand;
    const input = create.input as { SecretString: string };
    expect(input.SecretString.length).toBeGreaterThan(0);
  });
});
