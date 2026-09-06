import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LambdaFunctionURLEvent } from 'aws-lambda';
import {
  DAEMON_METRICS_CMD,
  DAEMON_STATUS_CMD,
  DAEMON_UNREACHABLE,
  parseDaemonMetrics,
  parseDaemonStatus,
} from '../lambda/shared/daemon';

// A representative /v1/metrics reply from the on-instance spinloop daemon —
// the Go side's metrics.Stats shape.
const daemonReply = JSON.stringify({
  state: 'running',
  runner: 'llamacpp',
  modelId: '/opt/llm/model/model.gguf',
  uptimeSeconds: 123,
  tokens: { running: 2, counter: 6020, promptTokens: 4096, generationTokens: 1024, requests: 17 },
  gpus: [
    {
      index: 0,
      name: 'NVIDIA L40S',
      utilization: 12,
      memoryUsed: 8589934592,
      memoryTotal: 48318382080,
      temperature: 42,
    },
  ],
  cpu: { utilization: 30 },
  memory: { total: 33020416512, used: 4294967296 },
  lastActiveAt: '2026-08-09T12:00:00Z',
  idleSeconds: 42,
});

describe('parseDaemonMetrics', () => {
  it('parses a daemon metrics reply', () => {
    const parsed = parseDaemonMetrics(daemonReply);
    expect(parsed).not.toBeNull();
    expect(parsed!.state).toBe('running');
    expect(parsed!.tokens).toEqual({
      running: 2,
      counter: 6020,
      promptTokens: 4096,
      generationTokens: 1024,
      requests: 17,
    });
    expect(parsed!.gpus).toHaveLength(1);
    expect(parsed!.gpus![0].memoryTotal).toBe(48318382080);
    expect(parsed!.cpu!.utilization).toBe(30);
    expect(parsed!.memory!.used).toBe(4294967296);
    expect(parsed!.lastActiveAt).toBe('2026-08-09T12:00:00Z');
    expect(parsed!.idleSeconds).toBe(42);
  });

  it('parses a reply with omitted stats (absent sources stay absent)', () => {
    const parsed = parseDaemonMetrics(JSON.stringify({ state: 'running' }));
    expect(parsed).not.toBeNull();
    expect(parsed!.tokens).toBeUndefined();
    expect(parsed!.gpus).toBeUndefined();
    expect(parsed!.history).toBeUndefined();
    expect(parsed!.lastActiveAt).toBeUndefined();
    expect(parsed!.idleSeconds).toBeUndefined();
  });

  it('parses the retained history the daemon reports', () => {
    const parsed = parseDaemonMetrics(
      JSON.stringify({
        state: 'running',
        cpu: { utilization: 62 },
        history: [
          { t: 1785000000, c: 12.5, m: 37.5, g: [{ i: 0, u: 88, m: 51.3 }] },
          { t: 1785000015, c: 62, m: 37.5, g: [{ i: 0, u: 95 }] },
        ],
      }),
    );
    expect(parsed).not.toBeNull();
    expect(parsed!.history).toEqual([
      { t: 1785000000, c: 12.5, m: 37.5, g: [{ i: 0, u: 88, m: 51.3 }] },
      { t: 1785000015, c: 62, m: 37.5, g: [{ i: 0, u: 95 }] },
    ]);
  });

  it('parses a stopped engine whose history survives the stop', () => {
    // The readings up to the stop are the point of the retention: they arrive
    // without any of the running-engine figures beside them.
    const parsed = parseDaemonMetrics(
      JSON.stringify({
        state: 'stopped',
        history: [{ t: 1785000015, c: 62, m: 37.5, g: [{ i: 0, u: 95 }] }],
        lastActiveAt: '2026-08-09T12:00:00Z',
        idleSeconds: 600,
      }),
    );
    expect(parsed).not.toBeNull();
    expect(parsed!.cpu).toBeUndefined();
    expect(parsed!.history).toHaveLength(1);
  });

  it('parses a stopped engine that still reports when it last worked', () => {
    // The record survives a stop, so this pair arrives without any of the
    // running-engine figures beside it.
    const parsed = parseDaemonMetrics(
      JSON.stringify({ state: 'stopped', lastActiveAt: '2026-08-09T12:00:00Z', idleSeconds: 600 }),
    );
    expect(parsed).not.toBeNull();
    expect(parsed!.tokens).toBeUndefined();
    expect(parsed!.lastActiveAt).toBe('2026-08-09T12:00:00Z');
    expect(parsed!.idleSeconds).toBe(600);
  });

  it('returns null for the unreachable marker', () => {
    expect(parseDaemonMetrics(`${DAEMON_UNREACHABLE}\n`)).toBeNull();
  });

  it('returns null for empty output', () => {
    expect(parseDaemonMetrics('')).toBeNull();
  });

  it('returns null for non-JSON output', () => {
    expect(parseDaemonMetrics('curl: (7) Failed to connect')).toBeNull();
  });

  it('returns null for JSON that is not a daemon reply', () => {
    expect(parseDaemonMetrics('42')).toBeNull();
    expect(parseDaemonMetrics('{"error":"missing bearer token"}')).toBeNull();
  });
});

describe('DAEMON_METRICS_CMD', () => {
  it('curls the loopback daemon and marks failure', () => {
    expect(DAEMON_METRICS_CMD).toContain('http://127.0.0.1:4242/v1/metrics');
    expect(DAEMON_METRICS_CMD).toContain(DAEMON_UNREACHABLE);
  });
});

// A representative /v1/status reply — the Go side's daemon.StatusResponse.
// This is what the idle check reads, so its parsing is tested alongside the
// metrics reply the stats path reads.
const statusReply = JSON.stringify({
  state: 'running',
  runner: 'llamacpp',
  model: '/opt/llm/model/model.gguf',
  uptimeSeconds: 1234,
  logPath: '/var/lib/spinloop/daemon/engine.log',
  lastActiveAt: '2026-08-09T12:00:00Z',
  idleSeconds: 42,
  version: '1.18.0',
});

describe('parseDaemonStatus', () => {
  it('parses a daemon status reply', () => {
    const parsed = parseDaemonStatus(statusReply);
    expect(parsed).not.toBeNull();
    expect(parsed!.state).toBe('running');
    expect(parsed!.lastActiveAt).toBe('2026-08-09T12:00:00Z');
    expect(parsed!.idleSeconds).toBe(42);
    expect(parsed!.version).toBe('1.18.0');
  });

  it('parses a reply from a daemon that has never run an engine', () => {
    const parsed = parseDaemonStatus(JSON.stringify({ state: 'idle' }));
    expect(parsed).not.toBeNull();
    expect(parsed!.lastActiveAt).toBeUndefined();
    expect(parsed!.idleSeconds).toBeUndefined();
  });

  it('returns null for the unreachable marker', () => {
    expect(parseDaemonStatus(`${DAEMON_UNREACHABLE}\n`)).toBeNull();
  });

  it('returns null for empty output', () => {
    expect(parseDaemonStatus('')).toBeNull();
  });

  it('returns null for non-JSON output', () => {
    expect(parseDaemonStatus('curl: (7) Failed to connect')).toBeNull();
  });

  it('returns null for JSON that is not a daemon reply', () => {
    expect(parseDaemonStatus('42')).toBeNull();
    expect(parseDaemonStatus('{"error":"missing bearer token"}')).toBeNull();
  });
});

describe('DAEMON_STATUS_CMD', () => {
  it('curls the loopback daemon and marks failure', () => {
    expect(DAEMON_STATUS_CMD).toContain('http://127.0.0.1:4242/v1/status');
    expect(DAEMON_STATUS_CMD).toContain(DAEMON_UNREACHABLE);
  });
});

// The stats Lambda: reports an environment's instance and engine metrics, and —
// while its Retain-Until tag is still a time in the future — the retention
// deadline itself, in every reply branch.

const LAMBDA_ENV = {
  TAG_KEY: 'cloud-vm-llm:managed',
  TAG_VALUE: 'true',
};

const findManagedInstance = vi.fn();
const readDeployConfig = vi.fn();
const runShellCommand = vi.fn();

vi.mock('../lambda/shared/aws', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/aws')>()),
  findManagedInstance: (...args: unknown[]) => findManagedInstance(...args),
  readDeployConfig: (...args: unknown[]) => readDeployConfig(...args),
  runShellCommand: (...args: unknown[]) => runShellCommand(...args),
}));

let handler: (event: LambdaFunctionURLEvent) => Promise<unknown>;

beforeAll(async () => {
  Object.assign(process.env, LAMBDA_ENV);
  ({ handler } = await import('../lambda/stats/index'));
});

function bodyOf(result: unknown): Record<string, unknown> {
  return JSON.parse((result as { statusCode: number; body: string }).body);
}

function statusOf(result: unknown): number {
  return (result as { statusCode: number }).statusCode;
}

function statsEvent(query: Record<string, string>) {
  return {
    queryStringParameters: query,
  } as unknown as LambdaFunctionURLEvent;
}

// The engine scrape is not what these cases assert on: the daemon answers
// nothing, so the reply carries no engine figures — only the control plane's
// own, which is where retainUntil lives.
beforeEach(() => {
  vi.clearAllMocks();
  readDeployConfig.mockResolvedValue({ runner: 'llamacpp', modelId: 'org/m' });
  runShellCommand.mockResolvedValue({ status: 'Failed', stdout: '' });
});

const futureTag = '2030-01-02T04:00:00.000Z';
const pastTag = '2020-01-02T04:00:00.000Z';

describe('retainUntil', () => {
  it('is present on a running instance whose tag is in the future', async () => {
    findManagedInstance.mockResolvedValue({
      instanceId: 'i-run',
      state: 'running',
      retainUntil: new Date(futureTag),
    });

    const result = await handler(statsEvent({ env: 'dev' }));
    const body = bodyOf(result);
    expect(statusOf(result)).toBe(200);
    expect(body.state).toBe('running');
    expect(body.retainUntil).toBe(futureTag);
  });

  it('is present on a stopped instance whose tag is in the future', async () => {
    findManagedInstance.mockResolvedValue({
      instanceId: 'i-stopped',
      state: 'stopped',
      retainUntil: new Date(futureTag),
    });

    const result = await handler(statsEvent({ env: 'dev' }));
    const body = bodyOf(result);
    expect(statusOf(result)).toBe(200);
    expect(body.state).toBe('stopped');
    expect(body.retainUntil).toBe(futureTag);
  });

  it('is absent when the tag has already passed', async () => {
    findManagedInstance.mockResolvedValue({
      instanceId: 'i-run',
      state: 'running',
      retainUntil: new Date(pastTag),
    });

    const result = await handler(statsEvent({ env: 'dev' }));
    const body = bodyOf(result);
    expect(statusOf(result)).toBe(200);
    expect(body).not.toHaveProperty('retainUntil');
  });

  it('is absent for an untagged instance', async () => {
    findManagedInstance.mockResolvedValue({ instanceId: 'i-run', state: 'running' });

    const result = await handler(statsEvent({ env: 'dev' }));
    const body = bodyOf(result);
    expect(statusOf(result)).toBe(200);
    expect(body).not.toHaveProperty('retainUntil');
  });

  it('is absent for an undeployed environment (no instance at all)', async () => {
    findManagedInstance.mockResolvedValue(null);

    const result = await handler(statsEvent({ env: 'dev' }));
    const body = bodyOf(result);
    expect(statusOf(result)).toBe(200);
    expect(body.state).toBe('undeployed');
    expect(body).not.toHaveProperty('retainUntil');
  });
});
