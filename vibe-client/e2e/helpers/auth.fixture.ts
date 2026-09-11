import { test as base, expect, type Page } from '@playwright/test';
import { TEST_OWNER } from './test-data';

/**
 * Custom test fixture that ensures the user is logged in.
 * Performs a fresh UI login for every test — no shared storageState —
 * to avoid refresh token reuse detection (the backend invalidates
 * all sessions when a token is reused).
 */
export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page }, use) => {
    await page.goto('/login');

    const submitBtn = page.getByRole('button', { name: 'Iniciar sesión' });
    await submitBtn.waitFor({ state: 'visible', timeout: 10_000 });

    await page.getByLabel('Email').fill(TEST_OWNER.email);
    await page.locator('#password').fill(TEST_OWNER.password);
    await submitBtn.click();
    await page.waitForURL(/\/(complexes|onboarding|dashboard)/, { timeout: 10_000 });

    // eslint-disable-next-line react-hooks/rules-of-hooks -- Playwright fixture API, not a React hook: `use` is the standard Playwright fixture callback parameter name, misidentified by the react-hooks plugin's naming heuristic in this non-React test-support file.
    await use(page);
  },
});

export { expect };
