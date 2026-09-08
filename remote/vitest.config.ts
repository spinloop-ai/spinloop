import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    // The seeder is its own project under seeder/ with its own tests; both
    // trees run in one `pnpm test` so there is a single lane to keep green.
    include: ['test/**/*.test.ts', 'seeder/test/**/*.test.ts'],
    environment: 'node',
    // Stack synth (with esbuild bundling of the Lambdas) is slow on first run,
    // and it happens in beforeAll — so the hooks get the same budget as the
    // tests that follow them.
    testTimeout: 120_000,
    hookTimeout: 120_000,
  },
});
