import { test, expect } from '@playwright/test';
import { TURNSTILE_TEST_TOKEN } from '../helpers/test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

// TST-08: nothing exercised the 401 -> refresh -> retry path ky.ts's
// `afterResponse` hook implements (src/shared/lib/ky.ts). Deliberately
// forces it by deleting the `access_token` cookie (HttpOnly, so this is the
// same thing an expired access token looks like to the app — the first
// request after it's gone gets a 401) and asserting the app recovers
// without a visible sign-out.
//
// Registers and logs in its own disposable owner rather than using the
// `authenticated` project's pooled sessions (auth.fixture.ts): a real
// refresh here rotates that account's refresh token, and any other spec
// still holding a pre-rotation copy of the same account's refresh token
// (from a static storageState file) would then look like a reused, revoked
// token to the backend. Kept self-contained instead.
test.describe('Access token refresh', () => {
  test('recovers from an expired access token without redirecting to /login', async ({ page }) => {
    const email = `e2e-token-refresh-${String(Date.now())}@test.com`;
    const password = 'TestPassword123!';

    const registerRes = await page.request.post(`${API}/auth/register`, {
      data: {
        email,
        password,
        first_name: 'Token',
        last_name: 'Refresh',
        phone: '+5491100000066',
        turnstile_token: TURNSTILE_TEST_TOKEN,
      },
    });
    expect(registerRes.status()).toBe(201);

    await page.goto('/login');
    await page.getByLabel('Email').fill(email);
    await page.locator('#password').fill(password);
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();
    await page.waitForURL('**/onboarding', { timeout: 15_000 });

    // Simulates the access token having expired: it's HttpOnly, so this is
    // as close as a test can get to "time passed" without actually waiting
    // out its 15-minute TTL. The refresh_token cookie (path
    // /api/v1/auth) is left in place — that's the one the recovery
    // is supposed to spend.
    await page.context().clearCookies({ name: 'access_token' });

    const refreshed = page.waitForResponse(
      (r) => r.url().includes('/auth/refresh') && r.request().method() === 'POST',
      { timeout: 15_000 },
    );
    await page.goto('/dashboard');
    const refreshResponse = await refreshed;
    expect(refreshResponse.status()).toBe(200);

    // Not necessarily still on /dashboard: this account has no complex yet,
    // and DashboardLayout's own routing for an owner with none is a
    // separate concern from what this test is about — only that the expired
    // access token recovered without a sign-out. /complexes (the owner's
    // "pick or create a complex" landing spot) is just as much a sign that
    // it worked as staying on /dashboard would be.
    await expect(page).not.toHaveURL(/\/login/);

    const cookies = await page.context().cookies();
    expect(cookies.some((c) => c.name === 'access_token')).toBe(true);
  });
});
