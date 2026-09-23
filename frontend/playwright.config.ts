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
        'owner-booking-create.spec.ts',
        'cancel-booking.spec.ts',
        'authorization.spec.ts',
        'tenant-isolation.spec.ts',
        'court-management.spec.ts',
        'dashboard.spec.ts',
        'clients.spec.ts',
        'settings.spec.ts',
        'navigation.spec.ts',
        'profile.spec.ts',
        'reports.spec.ts',
        'confirm-payment.spec.ts',
        'blocked-slots.spec.ts',
        'cash.spec.ts',
        'products.spec.ts',
        'sell.spec.ts',
        // Four owner specs that were in no project and therefore never ran.
        'blocked-slots-availability.spec.ts',
        'clients-search.spec.ts',
        'court-management-display.spec.ts',
        'settings-tabs.spec.ts',
        // Mixes authenticatedPage (dashboard/bookings/settings) and a plain,
        // unauthenticated page (login) in one file — belongs here for the
        // former; nothing about the latter requires the 'public' project.
        'a11y.spec.ts',
      ],
    },
    {
      // Platform-admin (superadmin) specs, using the TEST_ADMIN session
      // auth.setup.ts saves to e2e/.auth/admin.json (loaded by the
      // `adminPage` fixture in auth.fixture.ts — a single shared session is
      // fine here: low test volume, and a wholly different account/session
      // from the `authenticated` project's owner pool, so it can never race
      // that pool's refresh token.
      name: 'admin',
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'e2e/.auth/admin.json',
      },
      dependencies: ['setup'],
      testMatch: ['admin-users.spec.ts'],
    },
    {
      name: 'public',
      use: { ...devices['Desktop Chrome'] },
      testMatch: [
        'auth.spec.ts',
        'auth-validation.spec.ts',
        // Two unauthenticated specs that were in no project and never ran.
        'auth-navigation.spec.ts',
        'public-booking-form.spec.ts',
        'public-booking.spec.ts',
        'public-booking-keyboard.spec.ts',
        'public-cancel.spec.ts',
        'onboarding.spec.ts',
        // Registers and logs in its own dedicated, disposable owner account
        // rather than depending on `setup` — deliberately forces a token
        // refresh, which rotates that account's refresh token, so it must
        // never share a session with any other spec (see the spec's own
        // top-of-file comment).
        'token-refresh.spec.ts',
        // No login at all — a fresh, unauthenticated page/context is exactly
        // what its offline and service-worker-update checks need.
        'pwa.spec.ts',
      ],
    },
  ],

  webServer: {
    command: WEB_SERVER_COMMAND,
    url: BASE_URL,
    reuseExistingServer: !CI,
    // `make e2e` builds the app before serving it (see backend/scripts/e2e-run.sh):
    // the default dev server needs seconds, the production build needs a minute.
    timeout: 180_000,
  },
});
