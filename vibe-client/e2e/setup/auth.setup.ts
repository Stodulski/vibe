import { test as setup, expect } from '@playwright/test';
import { TEST_OWNER } from '../helpers/test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;
const AUTH_FILE = 'e2e/.auth/owner.json';

setup('create authenticated owner state', async ({ page }) => {
  // Register the test owner via API (auto-verified in development mode)
  const registerRes = await page.request.post(`${API}/auth/register`, {
    data: {
      email: TEST_OWNER.email,
      password: TEST_OWNER.password,
      first_name: TEST_OWNER.firstName,
      last_name: TEST_OWNER.lastName,
      phone: TEST_OWNER.phone,
    },
  });
  expect(registerRes.status()).toBe(201);

  // Login via the UI so cookies are properly set on the frontend domain
  await page.goto('/login');
  await page.getByLabel('Email').fill(TEST_OWNER.email);
  await page.locator('#password').fill(TEST_OWNER.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();

  // Wait for redirect to /complexes (authenticated state)
  await page.waitForURL('**/complexes', { timeout: 15_000 });

  // Save storage state (cookies + localStorage)
  await page.context().storageState({ path: AUTH_FILE });
});
