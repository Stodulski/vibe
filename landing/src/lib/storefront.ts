/**
 * Storefront switches shared by the pages the landing serves from the server.
 *
 * The app still serves the same public URLs until the cutover, so the Astro pages
 * must not compete with it in search yet. Flip STOREFRONT_INDEXABLE to true at the
 * cutover: the meta robots tag, the X-Robots-Tag header and the sitemap entries all
 * read this one constant.
 */
export const STOREFRONT_INDEXABLE = false;

/** Public origin of the storefront, used for canonical and JSON-LD URLs. */
export const STOREFRONT_ORIGIN = 'https://vibe.com.ar';

/**
 * The robots directive for a response with this status. Error answers are never
 * indexed, whatever the switch says.
 */
export function robotsDirective(status = 200): string {
  return STOREFRONT_INDEXABLE && status < 400 ? 'index, follow' : 'noindex';
}

/** A booking step: /c/<slug>/book and everything under it. */
const BOOK_PATH = /^\/c\/[^/]+\/book(?:\/|$)/;

export function isBookPath(pathname: string): boolean {
  return BOOK_PATH.test(pathname);
}

/**
 * The robots directive for a response on this path. Booking steps are private to one
 * visitor and never indexable, whatever the switch says. Everything else follows the
 * switch and the status.
 */
export function storefrontRobots(pathname: string, status = 200): string {
  return isBookPath(pathname) ? 'noindex' : robotsDirective(status);
}

/** The description the app ships in index.html, for pages with no complex of their own. */
export const DEFAULT_DESCRIPTION = 'Reserva canchas de pádel, tenis y fútbol de forma rápida y segura.';

/** The app's default share image, used when a complex has neither a cover nor a logo. */
export const DEFAULT_SHARE_IMAGE = 'https://app.vibe.com.ar/logo.png';

/**
 * The config the booking entries run with on the storefront. The API base is absolute,
 * and the privacy link points at this origin's /privacidad.
 */
export function storefrontBookingConfig(apiBaseUrl: string) {
  return {
    apiBaseUrl,
    publicSiteUrl: STOREFRONT_ORIGIN,
    privacyUrl: `${STOREFRONT_ORIGIN}/privacidad`,
  };
}

/**
 * The storefront's Content-Security-Policy, Report-Only first. It mirrors the app's
 * policy (frontend/vercel.json) with three changes, all because the storefront does not
 * use them:
 * - no challenges.cloudflare.com: the public booking flow loads no Turnstile widget;
 * - no *.r2.cloudflarestorage.com in connect-src: uploads live in the owner panel;
 * - no report-uri/report-to: the app's report endpoint is a rewrite in frontend/vercel.json,
 *   and this site has no such rewrite yet. Add it when the reports need to be collected.
 * `style-src 'unsafe-inline'` stays because React sets inline styles, and frame-ancestors
 * is kept in the header, where it applies (a meta tag cannot set it).
 */
export const STOREFRONT_CSP_REPORT_ONLY = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "form-action 'self'",
  "script-src 'self'",
  "style-src 'self' 'unsafe-inline'",
  "font-src 'self'",
  "img-src 'self' data: https://*.tile.openstreetmap.org https://*.r2.dev https://*.vibe.com.ar",
  "connect-src 'self' https://api.vibe.com.ar https://vibe.com.ar",
  "worker-src 'self' blob:",
  "manifest-src 'self'",
  "report-uri /_r/s",
  "report-to csp-endpoint",
].join('; ');

/**
 * The Reporting API group that `report-to csp-endpoint` names. It is sent as a header
 * on the storefront responses only, so the static pages' headers do not change. The
 * endpoint is this site's /_r/s, which vercel.json rewrites to the app's Sentry intake.
 */
export const STOREFRONT_REPORTING_ENDPOINTS = 'csp-endpoint="https://vibe.com.ar/_r/s"';
