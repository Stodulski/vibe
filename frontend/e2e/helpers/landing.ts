// The public pages (/c/<slug>, booking, cancel) are served by landing/ on its
// own origin, not by the app. playwright.config.ts starts that dev server and
// `make e2e` picks its port (see backend/scripts/e2e-run.sh).
const LANDING_URL = process.env.PLAYWRIGHT_LANDING_URL ?? 'http://localhost:4322';

/** Absolute URL of a public complex page, optionally a sub-route such as `/book/cancel`. */
export function landingComplexUrl(slug: string, suffix = ''): string {
  return `${LANDING_URL}/c/${slug}${suffix}`;
}
