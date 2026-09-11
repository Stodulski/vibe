import { test as base, expect, type Page } from '@playwright/test';
import { TEST_OWNER, TURNSTILE_TEST_TOKEN } from './test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

/**
 * Custom test fixture that ensures the user is logged in.
 * Performs a fresh login for every test — no shared storageState — to avoid
 * refresh token reuse detection (the backend invalidates all sessions when a
 * token is reused). The login goes through the page's own request context,
 * which shares its cookie jar, so the page then boots already signed in:
 * driving the login form cost two page loads per test for nothing the
 * auth.spec.ts login tests don't already cover.
 */
export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page }, use) => {
    const res = await page.context().request.post(`${API}/auth/login`, {
      data: {
        email: TEST_OWNER.email,
        password: TEST_OWNER.password,
        turnstile_token: TURNSTILE_TEST_TOKEN,
      },
    });
    if (!res.ok()) {
      throw new Error(`fixture login failed: ${String(res.status())} ${await res.text()}`);
    }
    // Hand the page over only once its boot has finished: the app refreshes
    // the session on load, and a spec navigating while that refresh is in
    // flight aborts it after the server rotated the token, which signs the
    // page out. The URL alone proves nothing here, `/complexes` is where it
    // already is; a successful `/auth/me` is the boot's last step.
    const bootedSignedIn = page.waitForResponse(
      (r) => r.url().includes('/auth/me') && r.request().method() === 'GET' && r.status() === 200,
      { timeout: 15_000 },
    );
    await page.goto('/complexes');
    await bootedSignedIn;
    // eslint-disable-next-line react-hooks/rules-of-hooks -- Playwright fixture API, not a React hook: `use` is the standard Playwright fixture callback parameter name, misidentified by the react-hooks plugin's naming heuristic in this non-React test-support file.
    await use(page);
  },
});

export { expect };
