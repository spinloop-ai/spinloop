import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

// The discovery contract the seed and start Lambdas both build on: one filter,
// so the two agree on which instance is a seed's compute. The gate's own tests
// stub findManagedInstances with this shape already assumed, so this is the one
// test that checks the wrapper builds the filter it claims to.

const findManagedInstances = vi.fn();

vi.mock('../lambda/shared/aws', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lambda/shared/aws')>()),
  findManagedInstances: (...args: unknown[]) => findManagedInstances(...args),
}));

let findSeedInstances: typeof import('../lambda/shared/seed/discovery').findSeedInstances;
let SEED_ID_TAG_KEY: string;
let SEED_TAG_VALUE: string;

beforeAll(async () => {
  ({ findSeedInstances } = await import('../lambda/shared/seed/discovery'));
  ({ SEED_ID_TAG_KEY, SEED_TAG_VALUE } = await import('../lambda/shared/seed/identity'));
});

describe('finding seed instances', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    findManagedInstances.mockResolvedValue([]);
  });

  it('selects on the seed tag value and nothing else when no id is given', async () => {
    await findSeedInstances('cloud-vm-llm');
    expect(findManagedInstances).toHaveBeenCalledWith('cloud-vm-llm', SEED_TAG_VALUE, []);
  });

  it('narrows to the seed id when one is given', async () => {
    await findSeedInstances('cloud-vm-llm', 'vllm--org-model--Q4_K_M');
    expect(findManagedInstances).toHaveBeenCalledWith('cloud-vm-llm', SEED_TAG_VALUE, [
      { Name: `tag:${SEED_ID_TAG_KEY}`, Values: ['vllm--org-model--Q4_K_M'] },
    ]);
  });
});
