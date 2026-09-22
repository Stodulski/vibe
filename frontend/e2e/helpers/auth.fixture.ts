import { test as base, expect, type Page } from '@playwright/test';
import { OWNER_SESSION_POOL_SIZE } from './test-data';

/**
 * Custom fixtures backed by the storageState files `auth.setup.ts` writes,
 * rather than a fresh login per test.
 *
 * The previous version logged in fresh for every single test specifically to
 * avoid refresh-token reuse detection (the backend invalidates every session
 * for a user the moment its refresh token is presented twice) — a shared
 * storageState, reused by many parallel tests, could trigger exactly that
 * the first time any one of them forced a token refresh. `authenticatedPage`
 * solves this the way TST-07's fix asks: instead of one owner session shared
 * by everyone, there are `OWNER_SESSION_POOL_SIZE` independent ones
 * (`e2e/.auth/owner-0.json` .. `owner-<N-1>.json`), and each test's worker
 * index picks one deterministically — two tests in different workers never
 * share a refresh token, and two tests in the same worker share the same
 * session the same way sequential logins always did (this is not the fresh
 * per-test login `authFixture` used to be; it's how a single logged-in tab
 * would behave across multiple page loads).
 */
/**
 * Opens a fresh context from `storageStatePath`, waits for the app's boot
 * check to answer 200, hands the page to the test, and — this is the part
 * a one-time `browser.newContext({ storageState: path })` load doesn't cover
 * — re-persists that context's (possibly rotated) cookies back to the same
 * file afterwards.
 *
 * Without the re-save, the first test anywhere in the run whose access
 * token has expired forces a real refresh (ky.ts's afterResponse hook), the
 * backend rotates that session's refresh token, and the file on disk still
 * holds the pre-rotation one — every later test that loads this same file
 * (same worker, sequentially) then presents an already-spent refresh token,
 * `bootstrapSession` treats the failed refresh as "no session" and returns
 * null without retrying (src/features/auth/hooks/useAuth.ts's catch), and
 * this fixture's `/auth/me` wait times out for the rest of the run. Writing
 * the rotation back closes that gap; two different workers finishing at the
 * same instant could still race the same file, but that is a rare,
 * self-correcting miss (the next test just re-reads whichever write landed
 * last) next to a permanent break.
 */
async function withPersistedSession(
  browser: import('@playwright/test').Browser,
  storageStatePath: string,
  landingUrl: string,
  use: (page: Page) => Promise<void>,
): Promise<void> {
  const context = await browser.newContext({ storageState: storageStatePath });
  const page = await context.newPage();
  // Hand the page over only once its boot has finished: the app refreshes
  // the session on load, and a spec navigating while that refresh is in
  // flight aborts it after the server rotated the token, which signs the
  // page out. The URL alone proves nothing here; a successful `/auth/me`
  // is the boot's last step.
  const bootedSignedIn = page.waitForResponse(
    (r) => r.url().includes('/auth/me') && r.request().method() === 'GET' && r.status() === 200,
    { timeout: 15_000 },
  );
  await page.goto(landingUrl);
  await bootedSignedIn;
  // eslint-disable-next-line react-hooks/rules-of-hooks -- Playwright fixture API, not a React hook: `use` is the standard Playwright fixture callback parameter name, misidentified by the react-hooks plugin's naming heuristic in this non-React test-support file.
  await use(page);
  await context.storageState({ path: storageStatePath });
  await context.close();
}

export const test = base.extend<{ authenticatedPage: Page; adminPage: Page }>({
  authenticatedPage: async ({ browser }, use, testInfo) => {
    const sessionIndex = testInfo.parallelIndex % OWNER_SESSION_POOL_SIZE;
    await withPersistedSession(browser, `e2e/.auth/owner-${String(sessionIndex)}.json`, '/dashboard', use);
  },

  // The single TEST_ADMIN (superadmin) session — one file is enough since
  // admin specs are few and never race the owner pool's refresh token (a
  // wholly different account, session and user).
  adminPage: async ({ browser }, use) => {
    await withPersistedSession(browser, 'e2e/.auth/admin.json', '/admin', use);
  },
});

export { expect };
