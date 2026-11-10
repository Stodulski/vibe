import { expect, type Page } from '@playwright/test';

// The public pages (/c/<slug>, booking, cancel) are served by landing/ on its
// own origin, not by the app. playwright.config.ts starts that dev server and
// `make e2e` picks its port (see backend/scripts/e2e-run.sh).
const LANDING_URL = process.env.PLAYWRIGHT_LANDING_URL ?? 'http://localhost:4322';

/** Absolute URL of a public complex page, optionally a sub-route such as `/book/cancel`. */
export function landingComplexUrl(slug: string, suffix = ''): string {
  return `${LANDING_URL}/c/${slug}${suffix}`;
}

/**
 * Waits until every Astro island on the page has hydrated: Astro removes the
 * `ssr` attribute from an island once its scripts ran. A click that lands on
 * the server-rendered markup before that does nothing.
 *
 * This replaces `waitForLoadState('networkidle')` on storefront pages: the
 * invisible Turnstile widget on the complex page keeps talking to Cloudflare,
 * so the network never goes idle there.
 */
export async function waitForStorefrontHydration(page: Page): Promise<void> {
  await expect(page.locator('astro-island').first()).toBeAttached();
  await expect(page.locator('astro-island[ssr]')).toHaveCount(0);
}
