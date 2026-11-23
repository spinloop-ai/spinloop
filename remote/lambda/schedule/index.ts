import type { LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';
import { errorName, requireEnv } from '../shared/aws';
import { environmentFrom } from '../shared/environments';
import { jsonResponse } from '../shared/http';
import {
  clearEnvSchedules,
  readSchedulesParam,
  reconcileSchedules,
  writeSchedulesParam,
} from '../shared/schedule-mirror';
import {
  nextRuns,
  ScheduleError,
  toSchedulerCron,
  validateSchedules,
  type Schedule,
} from '../shared/schedules';

const SCHEDULE_GROUP = requireEnv('SCHEDULE_GROUP');
// Where a fired schedule is delivered — fixed for this control plane, wired
// through to the mirror's reconcile.
const SCHEDULE_TARGETS = {
  schedulerRoleArn: requireEnv('SCHEDULER_ROLE_ARN'),
  startFnArn: requireEnv('START_FN_ARN'),
  stopFnArn: requireEnv('STOP_FN_ARN'),
};

/**
 * Function URL for an environment's schedules, named by `?env=`:
 * - GET reads the list and when each action next fires.
 * - PUT replaces the list with the JSON body `{"schedules": [...]}`.
 * - DELETE removes every schedule.
 *
 * The list is kept in SSM and mirrored into EventBridge Scheduler schedules.
 * Input is validated before anything is written, so a rejected request leaves
 * the existing schedules as they were. The AWS calls themselves live in
 * shared/schedule-mirror, which the stop Lambda's undeploy shares — one
 * environment's schedules are cleared the same way here and there.
 */
export async function handler(event: LambdaFunctionURLEvent): Promise<LambdaFunctionURLResult> {
  let env: string;
  try {
    env = environmentFrom(event.queryStringParameters);
  } catch (err) {
    return jsonResponse(400, { error: (err as Error).message });
  }
  const method = event.requestContext?.http?.method ?? 'GET';
  try {
    switch (method) {
      case 'GET':
        return report(env, await readSchedules(env));
      case 'PUT':
        return await replace(env, event);
      case 'DELETE':
        return await clear(env);
      default:
        return jsonResponse(405, { error: `unsupported method ${method}; accepted: GET, PUT, DELETE` });
    }
  } catch (err) {
    if (err instanceof ScheduleError) {
      return jsonResponse(400, { error: err.message });
    }
    console.log(JSON.stringify({ cmd: 'schedule', method, environment: env, error: errorName(err) }));
    throw err;
  }
}

function report(env: string, schedules: Schedule[]): LambdaFunctionURLResult {
  return jsonResponse(200, {
    environment: env,
    schedules,
    next: nextRuns(schedules, new Date()),
  });
}

async function replace(env: string, event: LambdaFunctionURLEvent): Promise<LambdaFunctionURLResult> {
  let body: unknown;
  try {
    body = JSON.parse(event.isBase64Encoded ? Buffer.from(event.body ?? '', 'base64').toString() : (event.body ?? ''));
  } catch {
    throw new ScheduleError('the body must be JSON: {"schedules": [...]}');
  }
  const schedules = validateSchedules((body as { schedules?: unknown } | null)?.schedules);
  // Everything that can be refused is refused above. Past here a failure is the
  // AWS side, and the stored list is already the intended one, so repeating the
  // request converges.
  const expressions = schedules.map((s) => toSchedulerCron(s.cron));
  if (schedules.length === 0) {
    return clear(env);
  }
  await writeSchedulesParam(env, JSON.stringify(schedules));
  await reconcileSchedules(env, SCHEDULE_GROUP, SCHEDULE_TARGETS, schedules, expressions);
  console.log(JSON.stringify({ cmd: 'schedule-set', environment: env, count: schedules.length }));
  return report(env, schedules);
}

async function clear(env: string): Promise<LambdaFunctionURLResult> {
  await clearEnvSchedules(env, SCHEDULE_GROUP);
  console.log(JSON.stringify({ cmd: 'schedule-clear', environment: env }));
  return report(env, []);
}

async function readSchedules(env: string): Promise<Schedule[]> {
  const stored = await readSchedulesParam(env);
  return validateSchedules(JSON.parse(stored ?? '[]'));
}
