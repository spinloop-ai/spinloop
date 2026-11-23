/**
 * Environments: the per-instance layer on top of the control-plane stack. The shared
 * stack (deployed once by `spinloop cloud bootstrap`) holds the Lambdas, the
 * weights bucket, roles and the VPC; an environment is created by the deploy
 * Lambda on demand and owns everything specific to one endpoint — its Elastic
 * IP (stable address), its security group (per-env allowed CIDR), its API-key
 * secret, and its deploy-config SSM parameter.
 *
 * Everything is keyed by the environment name, carried on resources as tags
 * (or encoded in names for the secret and SSM parameters), so one shared set
 * of Lambdas can find and drive every environment in the account.
 */

import {
  AllocateAddressCommand,
  AuthorizeSecurityGroupIngressCommand,
  CreateSecurityGroupCommand,
  DeleteSecurityGroupCommand,
  DescribeAddressesCommand,
  DescribeInstancesCommand,
  DescribeSecurityGroupsCommand,
  EC2Client,
  ReleaseAddressCommand,
  RevokeSecurityGroupIngressCommand,
} from '@aws-sdk/client-ec2';
import {
  CreateSecretCommand,
  DeleteSecretCommand,
  DescribeSecretCommand,
  GetSecretValueCommand,
  PutSecretValueCommand,
  SecretsManagerClient,
} from '@aws-sdk/client-secrets-manager';
import { randomBytes } from 'node:crypto';
import { errorName, listParametersUnder, sleep } from './aws';

const ec2 = new EC2Client({});
const secretsManager = new SecretsManagerClient({});

/** Tag naming which environment a resource (instance, EIP, SG) belongs to. */
export const ENV_TAG_KEY = 'cloud-vm-llm:env';

// A plain name: starts alphanumeric, then alphanumerics, dot, underscore or
// dash. Safe in an SSM parameter path, a Secrets Manager name, and a tag.
const ENV_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;

export function isValidEnvironmentName(name: string): boolean {
  return ENV_NAME.test(name);
}

/**
 * The environment a Function URL event addresses: the `env` query parameter,
 * else a JSON body's `environment` field (already parsed by the caller).
 * Mandatory — there is deliberately no default here: a control API that
 * silently assumed an environment could create or drive the wrong instance.
 * Conveniences like a default environment belong in the CLI, not in AWS.
 * Throws on a missing or invalid name.
 */
export function environmentFrom(
  query: Record<string, string | undefined> | undefined | null,
  bodyEnvironment?: unknown,
): string {
  const name = query?.env ?? (typeof bodyEnvironment === 'string' ? bodyEnvironment : '');
  if (!name) {
    throw new Error('missing node: pass ?env=<name> (or "environment" in the body)');
  }
  if (!isValidEnvironmentName(name)) {
    throw new Error(`invalid node name ${JSON.stringify(name)}`);
  }
  return name;
}

/** SSM parameter holding what an environment serves. */
export function deployConfigParam(env: string): string {
  return `/cloud-vm-llm/${env}/deploy-config`;
}

/** Secrets Manager name of an environment's API key. */
export function apiKeySecretName(env: string): string {
  return `cloud-vm-llm/${env}/api-key`;
}

/** An environment's base URL, from its Elastic IP. */
export function baseUrlFor(publicIp: string, port: number | string): string {
  return `http://${publicIp}:${port}/v1`;
}

export interface EnvEip {
  allocationId: string;
  publicIp: string;
}

/** Find an environment's Elastic IP by tag, if it has been allocated. */
export async function findEnvEip(env: string): Promise<EnvEip | null> {
  const result = await ec2.send(
    new DescribeAddressesCommand({
      Filters: [{ Name: `tag:${ENV_TAG_KEY}`, Values: [env] }],
    }),
  );
  const address = result.Addresses?.[0];
  if (!address?.AllocationId || !address.PublicIp) {
    return null;
  }
  return { allocationId: address.AllocationId, publicIp: address.PublicIp };
}

/** Allocate the environment's Elastic IP if it does not exist yet. */
export async function ensureEnvEip(env: string): Promise<EnvEip> {
  const existing = await findEnvEip(env);
  if (existing) {
    return existing;
  }
  const created = await ec2.send(
    new AllocateAddressCommand({
      Domain: 'vpc',
      TagSpecifications: [
        {
          ResourceType: 'elastic-ip',
          Tags: [
            { Key: ENV_TAG_KEY, Value: env },
            { Key: 'Name', Value: `cloud-vm-llm-${env}` },
          ],
        },
      ],
    }),
  );
  if (!created.AllocationId || !created.PublicIp) {
    throw new Error('AllocateAddress returned no allocation');
  }
  return { allocationId: created.AllocationId, publicIp: created.PublicIp };
}

/** Find an environment's security group by tag. */
export async function findEnvSecurityGroup(env: string): Promise<string | null> {
  const result = await ec2.send(
    new DescribeSecurityGroupsCommand({
      Filters: [{ Name: `tag:${ENV_TAG_KEY}`, Values: [env] }],
    }),
  );
  return result.SecurityGroups?.[0]?.GroupId ?? null;
}

/**
 * Ensure the environment's security group exists in the shared VPC and admits
 * exactly the allowed CIDR on the serving port. Ingress is per environment —
 * each environment scopes who can reach its own instance — so on a CIDR
 * change the old rules are revoked before the new one is authorised.
 */
export async function ensureEnvSecurityGroup(
  env: string,
  vpcId: string,
  port: number,
  allowedCidr: string,
): Promise<string> {
  let groupId = await findEnvSecurityGroup(env);
  if (!groupId) {
    const created = await ec2.send(
      new CreateSecurityGroupCommand({
        GroupName: `cloud-vm-llm-${env}`,
        Description: `cloud-vm-llm environment ${env} - ingress restricted to its allowed CIDR`,
        VpcId: vpcId,
        TagSpecifications: [
          {
            ResourceType: 'security-group',
            Tags: [
              { Key: ENV_TAG_KEY, Value: env },
              { Key: 'Name', Value: `cloud-vm-llm-${env}` },
            ],
          },
        ],
      }),
    );
    if (!created.GroupId) {
      throw new Error('CreateSecurityGroup returned no group id');
    }
    groupId = created.GroupId;
  }

  // Reconcile ingress to exactly [allowedCidr -> port].
  const described = await ec2.send(
    new DescribeSecurityGroupsCommand({ GroupIds: [groupId] }),
  );
  const current = described.SecurityGroups?.[0]?.IpPermissions ?? [];
  const wanted =
    current.length === 1 &&
    current[0].FromPort === port &&
    current[0].ToPort === port &&
    current[0].IpRanges?.length === 1 &&
    current[0].IpRanges[0].CidrIp === allowedCidr;
  if (wanted) {
    return groupId;
  }
  if (current.length > 0) {
    await ec2.send(
      new RevokeSecurityGroupIngressCommand({ GroupId: groupId, IpPermissions: current }),
    );
  }
  await ec2.send(
    new AuthorizeSecurityGroupIngressCommand({
      GroupId: groupId,
      IpPermissions: [
        {
          IpProtocol: 'tcp',
          FromPort: port,
          ToPort: port,
          IpRanges: [{ CidrIp: allowedCidr, Description: 'OpenAI-compatible API' }],
        },
      ],
    }),
  );
  return groupId;
}

/**
 * What ensureEnvApiKey did to the environment's API-key secret. The caller
 * reports it — the action, never the value: a rotated key invalidates the one
 * a live agent may still be holding, and that must be visible.
 */
export type EnvApiKeyAction = 'created' | 'rotated' | 'unchanged';

/**
 * Ensure the environment's API-key secret exists, generating a value on first
 * creation. The key is per environment, so revoking or rotating one endpoint
 * never touches another.
 *
 * A supplied key is stored: the secret is created with it when absent, and
 * set to it (PutSecretValue) when present — a deliberate rotation, invalidating
 * the previous key. Without one the secret is generated on first creation and
 * otherwise left alone: a redeploy must never mint a new key out from under a
 * live agent, and rotation is only ever explicit.
 */
export async function ensureEnvApiKey(env: string, providedKey?: string): Promise<EnvApiKeyAction> {
  const name = apiKeySecretName(env);
  let exists = false;
  try {
    await secretsManager.send(new DescribeSecretCommand({ SecretId: name }));
    exists = true;
  } catch (err) {
    if (errorName(err) !== 'ResourceNotFoundException') {
      throw err;
    }
  }
  if (providedKey) {
    if (exists) {
      await secretsManager.send(new PutSecretValueCommand({ SecretId: name, SecretString: providedKey }));
      return 'rotated';
    }
    await secretsManager.send(
      new CreateSecretCommand({
        Name: name,
        Description: `API key for the cloud-vm-llm environment ${env}`,
        SecretString: providedKey,
      }),
    );
    return 'created';
  }
  if (exists) {
    return 'unchanged';
  }
  await secretsManager.send(
    new CreateSecretCommand({
      Name: name,
      Description: `API key for the cloud-vm-llm environment ${env}`,
      SecretString: randomBytes(36).toString('base64url'),
    }),
  );
  return 'created';
}

/** Read an environment's API key. */
export async function readEnvApiKey(env: string): Promise<string> {
  const result = await secretsManager.send(
    new GetSecretValueCommand({ SecretId: apiKeySecretName(env) }),
  );
  return result.SecretString ?? '';
}

/**
 * Every stored deploy-config under the plane's SSM path, keyed by node
 * name. This is how the plane knows what it has deployed when it is asked for
 * an inventory without being named a node.
 */
export async function listEnvDeployConfigs(): Promise<Map<string, string>> {
  const configs = new Map<string, string>();
  for (const { name, value } of await listParametersUnder('/cloud-vm-llm/')) {
    const match = /^\/cloud-vm-llm\/([^/]+)\/deploy-config$/.exec(name);
    if (match) {
      configs.set(match[1], value);
    }
  }
  return configs;
}

/**
 * Every node named by the env tag on a resource — Elastic IPs, security
 * groups and instances alike. A resource tagged for a node belongs to
 * one whether or not its deploy ever wrote state, so the tag values complete
 * the picture the deploy-config parameters start.
 */
export async function taggedEnvNames(): Promise<string[]> {
  // A `*` value on a tag filter matches any value: everything that carries
  // the env tag, which is exactly what this plane names.
  const filters = [{ Name: `tag:${ENV_TAG_KEY}`, Values: ['*'] }];
  const names = new Set<string>();
  const envOf = (tags?: { Key?: string; Value?: string }[]): string | undefined =>
    tags?.find((t) => t.Key === ENV_TAG_KEY)?.Value;
  const addresses = await ec2.send(new DescribeAddressesCommand({ Filters: filters }));
  for (const address of addresses.Addresses ?? []) {
    const env = envOf(address.Tags);
    if (env) names.add(env);
  }
  const groups = await ec2.send(new DescribeSecurityGroupsCommand({ Filters: filters }));
  for (const group of groups.SecurityGroups ?? []) {
    const env = envOf(group.Tags);
    if (env) names.add(env);
  }
  const instances = await ec2.send(new DescribeInstancesCommand({ Filters: filters }));
  for (const instance of instances.Reservations?.flatMap((r) => r.Instances ?? []) ?? []) {
    const env = envOf(instance.Tags);
    if (env) names.add(env);
  }
  return [...names];
}

/** Release an Elastic IP by allocation id. */
export async function releaseEip(allocationId: string): Promise<void> {
  await ec2.send(new ReleaseAddressCommand({ AllocationId: allocationId }));
}

/**
 * How often to retry a security-group delete before giving up: an instance
 * that was just terminated can hold its group for a few seconds on the way out.
 */
const SG_DELETE_ATTEMPTS = 6;

/**
 * Delete a security group by id, tolerating the brief window in which just-
 * terminated instances still depend on it. Any other error — or a dependency
 * that outlasts the retries — throws: an undeploy that left a group behind
 * must say so rather than report a clean teardown.
 */
export async function deleteSecurityGroup(groupId: string, retryDelayMs = 2000): Promise<void> {
  for (let attempt = 1; ; attempt++) {
    try {
      await ec2.send(new DeleteSecurityGroupCommand({ GroupId: groupId }));
      return;
    } catch (err) {
      if (errorName(err) !== 'DependencyViolation' || attempt >= SG_DELETE_ATTEMPTS) {
        throw err;
      }
      await sleep(retryDelayMs);
    }
  }
}

/**
 * Delete the node's API-key secret, immediately and past recovery —
 * the node itself is going (or has gone). Returns whether the secret
 * was there; with dryRun it only reports what it finds.
 */
export async function deleteEnvApiKey(env: string, dryRun = false): Promise<boolean> {
  const name = apiKeySecretName(env);
  try {
    await secretsManager.send(new DescribeSecretCommand({ SecretId: name }));
  } catch (err) {
    if (errorName(err) === 'ResourceNotFoundException') {
      return false;
    }
    throw err;
  }
  if (!dryRun) {
    await secretsManager.send(new DeleteSecretCommand({ SecretId: name, ForceDeleteWithoutRecovery: true }));
  }
  return true;
}

