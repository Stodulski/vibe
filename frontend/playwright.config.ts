import { defineConfig, devices } from '@playwright/test';

const CI = !!process.env.CI;

// Defaults preserve today's behaviour (dev Vite on :5173) when unset. `make
// e2e` in backend sets these to an isolated port (:5174) and an
// isolated-backend web-server command, so a local `pnpm test:e2e` never
// touches the developer's running dev Vite/API by accident.
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:5173';
const WEB_SERVER_COMMAND = process.env.PLAYWRIGHT_WEB_SERVER_COMMAND ?? 'pnpm exec vite --mode e2e';

export default defineConfig({
  testDir: './e2e/specs',
  fullyParallel: true,
  forbidOnly: CI,
  retries: CI ? 1 : 0,
  workers: CI ? 1 : undefined,
  reporter: CI ? 'html' : 'list',
  outputDir: './e2e-results',

  use: {
    baseURL: BASE_URL,
    locale: 'es-AR',
    timezoneId: 'America/Argentina/Buenos_Aires',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  globalSetup: './e2e/setup/global.setup.ts',

  projects: [
    {
      name: 'setup',
      testDir: './e2e/setup',
      testMatch: 'auth.setup.ts',
    },
    {
      name: 'authenticated',
      use: {
        ...devices['Desktop Chrome'],
      },
      dependencies: ['setup'],
      testMatch: [
        'owner-booking.spec.ts',
        'court-management.spec.ts',
        'dashboard.spec.ts',
        'clients.spec.ts',
        'settings.spec.ts',
        'navigation.spec.ts',
        'profile.spec.ts',
        'reports.spec.ts',
        'confirm-payment.spec.ts',
        'blocked-slots.spec.ts',
      ],
    },
    {
      name: 'public',
      use: { ...devices['Desktop Chrome'] },
      testMatch: [
        'auth.spec.ts',
        'auth-validation.spec.ts',
        'public-booking.spec.ts',
        'public-cancel.spec.ts',
        'onboarding.spec.ts',
      ],
    },
  ],

  webServer: {
    command: WEB_SERVER_COMMAND,
    url: BASE_URL,
    reuseExistingServer: !CI,
    timeout: 30_000,
  },
});
