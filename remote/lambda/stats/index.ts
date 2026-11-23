import type { LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';
import {
  errorName,
  findManagedInstance,
  findParameter,
  type InstanceInfo,
  readDeployConfig,
  requireEnv,
  runShellCommand,
} from '../shared/aws';
import { type DeployConfig, parseDeployConfig } from '../shared/deploy-config';
import {
  DAEMON_METRICS_CMD,
  DAEMON_STATUS_CMD,
  parseDaemonMetrics,
  parseDaemonStatus,
} from '../shared/daemon';
import {
  baseUrlFor,
  deployConfigParam,
  ENV_TAG_KEY,
  environmentFrom,
  findEnvEip,
  listEnvDeployConfigs,
  taggedEnvNames,
} from '../shared/environments';
import { jsonResponse } from '../shared/http';
import { type StatsResult } from '../shared/stats';

const TAG_KEY = requireEnv('TAG_KEY');
const TAG_VALUE = requireEnv('TAG_VALUE');
const PORT = Number(requireEnv('ENGINE_PORT'));

/** Narrow instance discovery to one environment's instance. */
function envFilter(env: string) {
  return [{ Name: `tag:${ENV_TAG_KEY}`, Values: [env] }];
}

/**
 * The retention deadline to carry in a reply, or nothing: present only while
 * the instance's Retain-Until tag is still a time in the future. A passed
 * deadline keeps nothing, so the reply drops it there; an untagged instance and
 * a not-yet-deployed environment have none. The judgement lives here, not in the
 * formatters, so a reader of the reply need not re-check the clock — and it
 * applies to every reply branch, since a stopped, retained instance still
 * reports its deadline.
 */
function retainUntilIfActive(instance: InstanceInfo | null): string | undefined {
  const until = instance?.retainUntil;
  if (until && until.getTime() > Date.now()) {
    return until.toISOString();
  }
  return undefined;
}

/**
 * The stats Lambda called by `spinloop cloud metrics`. The control plane
 * contributes what only it knows — environment, instance id/type, uptime
 * since launch — and everything measured (engine token counters, GPU, CPU,
 * RAM) comes from the on-instance spinloop daemon's /v1/metrics, fetched with
 * one SSM curl; the daemon's version rides along from /v1/status in a
 * parallel second. Collection itself lives in spinloop's internal/metrics.
 *
 * Without an `env` query parameter the same URL answers the plane's
 * inventory instead — every environment it holds — because an inventory is a
 * question about the plane, not about one endpoint. Older CLIs always send
 * `env`, so their behaviour is unchanged.
 */
export async function handler(event: LambdaFunctionURLEvent): Promise<LambdaFunctionURLResult> {
  if (event.queryStringParameters?.env === undefined) {
    return inventory();
  }
  let env: string;
  try {
    env = environmentFrom(event.queryStringParameters);
  } catch (err) {
    return jsonResponse(400, { error: (err as Error).message });
  }

  // Read deploy config for runner and model info.
  let deployConfig: DeployConfig;
  try {
    deployConfig = await readDeployConfig(deployConfigParam(env));
  } catch (err) {
    return jsonResponse(400, {
      error: `cannot read deploy config: ${(err as Error).message}. Run \`spinloop cloud deploy\` first.`,
    });
  }

  // Find the instance.
  const instance = await findManagedInstance(TAG_KEY, TAG_VALUE, envFilter(env));

  if (!instance || instance.state !== 'running') {
    const result: StatsResult = {
      environment: env,
      state: instance?.state ?? (instance ? 'stopped' : 'undeployed'),
      runner: deployConfig.runner,
      modelId: deployConfig.modelId,
      servedName: deployConfig.servedModelName,
    };
    // A stopped environment can still be retained: its deadline is the control
    // plane's, not the engine's, so it rides this branch too.
    result.retainUntil = retainUntilIfActive(instance);
    return jsonResponse(200, result);
  }

  const result: StatsResult = {
    environment: env,
    state: 'running',
    instanceId: instance.instanceId,
    runner: deployConfig.runner,
    modelId: deployConfig.modelId,
    servedName: deployConfig.servedModelName,
  };
  if (instance.instanceType) {
    result.instanceType = instance.instanceType;
  }
  // The retention deadline is the instance's, so it is set here, with the
  // other instance facts, rather than alongside the daemon's figures.
  result.retainUntil = retainUntilIfActive(instance);
  // Uptime from launch time — the instance's, not the engine's, since cost
  // estimation multiplies it by the on-demand price.
  if (instance.launchTime) {
    result.uptimeSeconds = Math.floor((Date.now() - instance.launchTime.getTime()) / 1000);
  }

  const errors: string[] = [];
  try {
    // Both scrapes go over SSM, so they run side by side — the same
    // composition the start Lambda uses for its daemon probe — keeping the
    // handler within its existing latency budget.
    const [metrics, version] = await Promise.all([
      runShellCommand(instance.instanceId, DAEMON_METRICS_CMD, 30),
      readDaemonVersion(instance.instanceId),
    ]);
    const daemon = metrics.status === 'Success' ? parseDaemonMetrics(metrics.stdout) : null;
    if (daemon) {
      result.tokens = daemon.tokens;
      result.gpus = daemon.gpus;
      result.cpu = daemon.cpu;
      result.memory = daemon.memory;
      // Relayed verbatim, like the activity pair below: the daemon decides
      // what counts — the window, the cadence, the retention — and the relay
      // does not reshape the readings.
      result.history = daemon.history;
      // Relayed verbatim: the daemon decides what counts as activity, and an
      // absent pair means "nothing to report" rather than "idle since boot".
      result.lastActiveAt = daemon.lastActiveAt;
      result.idleSeconds = daemon.idleSeconds;
      if (daemon.errors?.length) {
        errors.push(...daemon.errors);
      }
    } else {
      errors.push('daemon: unreachable or unrecognisable metrics reply');
    }
    if (version) {
      result.version = version;
    }
  } catch (err) {
    errors.push(`daemon: ${errorName(err)}`);
  }
  if (errors.length > 0) {
    result.errors = errors;
  }

  return jsonResponse(200, result);
}

/**
 * Ask the instance's daemon which spinloop build it runs, for the report's
 * version line. Every failure — SSM error, unreachable daemon, an older
 * daemon without the field — yields undefined, so a working metrics reply
 * never degrades because the version is an add-on, not a health signal.
 */
async function readDaemonVersion(instanceId: string): Promise<string | undefined> {
  try {
    const result = await runShellCommand(instanceId, DAEMON_STATUS_CMD, 10);
    if (result.status !== 'Success') {
      return undefined;
    }
    return parseDaemonStatus(result.stdout)?.version || undefined;
  } catch (err) {
    console.log(JSON.stringify({ phase: 'daemon-version', error: errorName(err) }));
    return undefined;
  }
}

/** One environment in the inventory reply. Unknown facts are empty strings; `partial` marks an environment the plane could name but not fully read. */
interface InventoryEntry {
  environment: string;
  deployedBy: string;
  deployedAt: string;
  runner: string;
  modelId: string;
  servedName: string;
  baseUrl: string;
  state: string;
  instanceType: string;
  instanceId: string;
  partial: boolean;
}

/**
 * `GET {statsUrl}` with no `env`: every environment this plane holds. The
 * names are the union of the deploy-config parameters under `/cloud-vm-llm/`
 * and the `cloud-vm-llm:env` tag values on Elastic IPs, security groups and
 * instances — an environment with a config but no instance is still listed,
 * and one tagged but never deployed cannot go unmentioned. Each
 * per-environment read is best-effort: one failure marks that entry partial
 * and is noted in `errors`, never failing the listing. A plane with nothing
 * answers `{"environments":[]}`.
 */
async function inventory(): Promise<LambdaFunctionURLResult> {
  const errors: string[] = [];
  const names = new Set<string>();
  const storedConfigs = new Map<string, string>();
  try {
    for (const [env, raw] of await listEnvDeployConfigs()) {
      names.add(env);
      storedConfigs.set(env, raw);
    }
  } catch (err) {
    errors.push(`deploy-configs: ${errorName(err)}`);
  }
  try {
    for (const env of await taggedEnvNames()) {
      names.add(env);
    }
  } catch (err) {
    errors.push(`environment tags: ${errorName(err)}`);
  }

  const environments: InventoryEntry[] = [];
  for (const env of [...names].sort()) {
    environments.push(await inventoryEntry(env, storedConfigs.get(env), errors));
  }
  return jsonResponse(200, { environments, ...(errors.length > 0 ? { errors } : {}) });
}

/** Assemble one inventory entry. Never throws: a failed read marks the entry partial and notes the error instead. */
async function inventoryEntry(
  env: string,
  storedConfig: string | undefined,
  errors: string[],
): Promise<InventoryEntry> {
  let partial = false;
  let config: DeployConfig | null = null;
  try {
    // Reuse the value the enumeration read when it has one; an environment
    // known only from a tag that has no parameter was never deployed, which
    // is a fact, not a read failure.
    const raw = storedConfig ?? (await findParameter(deployConfigParam(env)));
    if (raw !== null) {
      config = parseDeployConfig(raw);
    }
  } catch (err) {
    partial = true;
    errors.push(`${env}: deploy-config ${errorName(err)}`);
  }
  let publicIp = '';
  try {
    publicIp = (await findEnvEip(env))?.publicIp ?? '';
  } catch (err) {
    partial = true;
    errors.push(`${env}: elastic IP ${errorName(err)}`);
  }
  let instance: InstanceInfo | null = null;
  try {
    instance = await findManagedInstance(TAG_KEY, TAG_VALUE, envFilter(env));
  } catch (err) {
    partial = true;
    errors.push(`${env}: instance lookup ${errorName(err)}`);
  }
  return {
    environment: env,
    deployedBy: config?.deployedBy ?? '',
    deployedAt: config?.deployedAt ?? '',
    runner: config?.runner ?? '',
    modelId: config?.modelId ?? '',
    servedName: config?.servedModelName ?? '',
    baseUrl: publicIp ? baseUrlFor(publicIp, PORT) : '',
    // No instance is the stopped plane-state; the inventory is about what the
    // control plane holds, not what EC2 happens to be carrying right now.
    state: instance?.state ?? 'stopped',
    instanceType: instance?.instanceType ?? '',
    instanceId: instance?.instanceId ?? '',
    partial,
  };
}
