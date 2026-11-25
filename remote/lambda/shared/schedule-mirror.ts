/**
 * The AWS half of a node's schedules: the SSM list parameter and its
 * EventBridge Scheduler mirror.
 *
 * The schedule Lambda owns the lifecycle (set, read, clear) — but clearing is
 * needed by more than it. `spinloop cloud undeploy` must take a node's
 * schedules out with the rest of it, and doing that any other way would leave
 * orphaned schedules firing at a node that no longer exists. So the
 * mirror calls live here, where both Lambdas import the same clear logic. The
 * pure half — naming, cron, validation — stays in ./schedules.
 */

import {
  CreateScheduleCommand,
  DeleteScheduleCommand,
  ListSchedulesCommand,
  SchedulerClient,
  UpdateScheduleCommand,
} from '@aws-sdk/client-scheduler';
import {
  DeleteParameterCommand,
  GetParameterCommand,
  PutParameterCommand,
  SSMClient,
} from '@aws-sdk/client-ssm';
import { errorName } from './aws';
import {
  SCHEDULE_EVENT_SOURCE,
  schedulerNamePrefix,
  schedulesParam,
  type Schedule,
  type ScheduledRunEvent,
} from './schedules';

const ssm = new SSMClient({});
const scheduler = new SchedulerClient({});

/** Where a fired schedule is delivered: the start and stop Lambda ARNs, and the role Scheduler assumes to invoke them. */
export interface ScheduleTargets {
  schedulerRoleArn: string;
  startFnArn: string;
  stopFnArn: string;
}

/** What the mirror holds for one node: whether the list is stored, and which Scheduler schedule names exist. */
export interface EnvSchedules {
  parameter: boolean;
  names: string[];
}

/** The stored schedule list, or null when nothing is stored. */
export async function readSchedulesParam(env: string): Promise<string | null> {
  try {
    const out = await ssm.send(new GetParameterCommand({ Name: schedulesParam(env) }));
    return out.Parameter?.Value ?? '';
  } catch (err) {
    if (errorName(err) === 'ParameterNotFound') {
      return null;
    }
    throw err;
  }
}

/** Overwrite the stored schedule list. */
export async function writeSchedulesParam(env: string, value: string): Promise<void> {
  await ssm.send(
    new PutParameterCommand({
      Name: schedulesParam(env),
      Value: value,
      Type: 'String',
      Overwrite: true,
    }),
  );
}

/** Remove the stored list. True when it existed. */
async function clearSchedulesParam(env: string): Promise<boolean> {
  try {
    await ssm.send(new DeleteParameterCommand({ Name: schedulesParam(env) }));
    return true;
  } catch (err) {
    if (errorName(err) !== 'ParameterNotFound') {
      throw err;
    }
    return false;
  }
}

/** The Scheduler schedule names the node currently has, across every page. */
async function listScheduleNames(env: string, group: string): Promise<string[]> {
  const names: string[] = [];
  let token: string | undefined;
  do {
    const page = await scheduler.send(
      new ListSchedulesCommand({
        GroupName: group,
        NamePrefix: schedulerNamePrefix(env),
        NextToken: token,
      }),
    );
    for (const s of page.Schedules ?? []) {
      if (s.Name) {
        names.push(s.Name);
      }
    }
    token = page.NextToken;
  } while (token);
  return names;
}

/** What the mirror holds for the node — reads only, safe beside a dry run. */
export async function describeEnvSchedules(env: string, group: string): Promise<EnvSchedules> {
  const parameter = (await readSchedulesParam(env)) !== null;
  return { parameter, names: await listScheduleNames(env, group) };
}

/** Make the node's Scheduler schedules match the list: create or update each, then remove the rest. */
export async function reconcileSchedules(
  env: string,
  group: string,
  targets: ScheduleTargets,
  schedules: Schedule[],
  expressions: string[],
): Promise<void> {
  const prefix = schedulerNamePrefix(env);
  const existing = new Set(await listScheduleNames(env, group));
  const wanted = new Set<string>();
  for (const [i, schedule] of schedules.entries()) {
    const name = `${prefix}${i + 1}`;
    wanted.add(name);
    const input: ScheduledRunEvent = {
      source: SCHEDULE_EVENT_SOURCE,
      action: schedule.action,
      environment: env,
    };
    const params = {
      Name: name,
      GroupName: group,
      ScheduleExpression: expressions[i],
      ScheduleExpressionTimezone: schedule.timezone,
      FlexibleTimeWindow: { Mode: 'OFF' as const },
      Target: {
        Arn: schedule.action === 'start' ? targets.startFnArn : targets.stopFnArn,
        RoleArn: targets.schedulerRoleArn,
        Input: JSON.stringify(input),
      },
    };
    await scheduler.send(existing.has(name) ? new UpdateScheduleCommand(params) : new CreateScheduleCommand(params));
  }
  for (const name of existing) {
    if (!wanted.has(name)) {
      await scheduler.send(new DeleteScheduleCommand({ Name: name, GroupName: group }));
    }
  }
}

/**
 * Remove everything: the stored list and every Scheduler schedule of the
 * node. Returns what was there, so a caller taking the node
 * apart can report exactly what it released.
 */
export async function clearEnvSchedules(env: string, group: string): Promise<EnvSchedules> {
  const parameter = await clearSchedulesParam(env);
  const names = await listScheduleNames(env, group);
  for (const name of names) {
    await scheduler.send(new DeleteScheduleCommand({ Name: name, GroupName: group }));
  }
  return { parameter, names };
}
