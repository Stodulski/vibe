import { execFileSync } from 'child_process';
import { test as setup, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { TEST_OWNER, TEST_ADMIN, OWNER_SESSION_POOL_SIZE, TURNSTILE_TEST_TOKEN } from '../helpers/test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

interface Credentials {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
  phone: string;
}

/** Registers an owner account via the public API, tolerating "already exists" (409). */
async function registerOwner(request: APIRequestContext, owner: Credentials): Promise<void> {
  const res = await request.post(`${API}/auth/register`, {
    data: {
      email: owner.email,
      password: owner.password,
      first_name: owner.firstName,
      last_name: owner.lastName,
      phone: owner.phone,
      turnstile_token: TURNSTILE_TEST_TOKEN,
    },
  });
  expect([201, 409]).toContain(res.status());
}

/**
 * Promotes a just-registered account to `superadmin` directly in the E2E
 * database — the public register endpoint always creates `owner` accounts
 * (internal/auth/handlers.go), so there is no API path to a platform admin.
 * Same "seed via psql" pattern as `ApiHelper.setupFullComplex`'s fake
 * MercadoPago connection.
 */
function promoteToSuperadmin(email: string): void {
  execFileSync(
    'psql',
    [
      '-h',
      process.env.E2E_DB_HOST ?? 'localhost',
      '-p',
      process.env.E2E_DB_PORT ?? '5433',
      '-U',
      process.env.E2E_DB_USER ?? 'vibe',
      '-d',
      process.env.E2E_DB_NAME ?? 'vibe_e2e',
      '-c',
      `UPDATE users SET role = 'superadmin' WHERE email = '${email}';`,
    ],
    {
      stdio: 'pipe',
      env: { ...process.env, PGPASSWORD: process.env.E2E_DB_PASSWORD ?? 'vibe_e2e' },
    },
  );
}

async function loginAndSaveState(
  page: Page,
  credentials: Pick<Credentials, 'email' | 'password'>,
  storageStatePath: string,
  // RootRedirect (src/app/router/RootRedirect.tsx) sends a superadmin to
  // /admin and everyone else to /complexes — waiting on the wrong one hangs
  // until this call's own timeout regardless of how long the real
  // navigation took.
  postLoginUrlPattern: string | RegExp = '**/complexes',
): Promise<void> {
  await page.goto('/login');
  await page.getByLabel('Email').fill(credentials.email);
  await page.locator('#password').fill(credentials.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await page.waitForURL(postLoginUrlPattern, { timeout: 15_000 });
  await page.context().storageState({ path: storageStatePath });
}

// One independent login session per pool slot (see OWNER_SESSION_POOL_SIZE's
// doc comment in test-data.ts) — `authenticatedPage` (auth.fixture.ts)
// distributes tests across these by worker index instead of every worker
// sharing one refresh token. `fullyParallel: true` (playwright.config.ts)
// means these can run before, after, or alongside one another in different
// workers, so each registers the (idempotent, 409-tolerant) account itself
// rather than depending on a separate "create the account" test's ordering.
for (let i = 0; i < OWNER_SESSION_POOL_SIZE; i++) {
  setup(`save TEST_OWNER session ${String(i)}`, async ({ page }) => {
    await registerOwner(page.request, TEST_OWNER);
    await loginAndSaveState(page, TEST_OWNER, `e2e/.auth/owner-${String(i)}.json`);
  });
}

setup('create and save the TEST_ADMIN (superadmin) session', async ({ page }) => {
  await registerOwner(page.request, TEST_ADMIN);
  promoteToSuperadmin(TEST_ADMIN.email);
  await loginAndSaveState(page, TEST_ADMIN, 'e2e/.auth/admin.json', '**/admin');
});
