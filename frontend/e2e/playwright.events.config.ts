import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, devices } from '@playwright/test';
import base from '../playwright.config';

/**
 * Runs only the three specs for turnos fijos, canchas libres and torneos
 * (`fixed-bookings`, `open-play-events`, `tournaments`). They each provision
 * their own owner and complex (see helpers/events-fixtures.ts), so they need
 * neither the shared `setup` project nor a single-worker project.
 *
 *   E2E_SPECS="--config=e2e/playwright.events.config.ts" make e2e   (backend/)
 *
 * `playwright.config.ts` lists the other specs by project; adding the three
 * files to its `authenticated` project's `testMatch` runs them in the normal
 * suite as well, and makes this file unnecessary.
 */
const HERE = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  ...base,
  testDir: path.resolve(HERE, 'specs'),
  globalSetup: path.resolve(HERE, 'setup/global.setup.ts'),
  outputDir: path.resolve(HERE, '../e2e-results'),
  // Playwright starts web servers from the config's own directory: keep frontend/ as their cwd.
  webServer: (Array.isArray(base.webServer) ? base.webServer : []).map((server) => ({
    ...server,
    cwd: path.resolve(HERE, '..'),
  })),
  projects: [
    {
      name: 'events',
      use: { ...devices['Desktop Chrome'] },
      testMatch: ['fixed-bookings.spec.ts', 'open-play-events.spec.ts', 'tournaments.spec.ts'],
    },
  ],
});
