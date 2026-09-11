import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'happy-dom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: true,
    // One environment per file.
    //
    // Sharing one per worker ran the suite in 35s instead of 186s, but made it
    // order-dependent: one or two tests failed per run, a different set each
    // time, as module state leaked between files. Resetting mocks was not
    // enough — the leak is module-level state, not call counts.
    //
    // A suite that is green half the time is not a gate, which is why the CI
    // job that runs it was marked continue-on-error. Three minutes for a
    // deterministic result is the better trade; watch mode only reruns the
    // files you touched.
    isolate: true,
    // Half the cores, not all of them. Each isolated file boots its own
    // happy-dom, and four workers on a 16 GB machine got the run killed for
    // memory when anything else was running. Two workers finish in about ten
    // minutes with a 3 GB heap per worker (see `test` in package.json).
    maxWorkers: '50%',
    clearMocks: true,
    restoreMocks: true,
    testTimeout: 10000,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'text-summary', 'lcov'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}', 'src/**/*.spec.{ts,tsx}', 'src/**/types/**', 'src/**/*.d.ts', 'src/main.tsx'],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
