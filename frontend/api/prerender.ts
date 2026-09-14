/// <reference types="node" />

/**
 * Vercel Function behind the crawler-prerender rewrite in `vercel.json`.
 *
 * The rewrite's `has: [{ type: 'header', key: 'user-agent', ... }]` already
 * keeps human traffic from ever reaching this file, so this file does not
 * check the user agent itself — that job stays at the routing layer. What
 * this file owns is the failure semantics a straight rewrite to the backend
 * would lose: a bounded fetch budget, and an answer a crawler knows how to
 * act on for every way the backend can fail.
 *
 * Fetched rather than left as a bare proxy so the answer can be inspected
 * before it reaches the crawler: a crawler indexes whatever it gets with a
 * 200, so handing it the backend's own error page (or a stale gateway 404)
 * on failure would either poison the index with garbage or de-index a venue
 * that still exists. 503 is the one status a crawler retries instead of
 * dropping the URL, so every transient failure becomes a 503 with
 * Retry-After rather than a 500 or a silently wrong 200.
 */

/**
 * The single-segment paths the app owns at the root.
 *
 * This used to be a SKIP_PREFIXES denylist that anything unlisted fell
 * through, and it drifted: /profile, /reports and /admin were added to the
 * router and never added there, so a crawler on /profile was rewritten to
 * the prerender endpoint and served the backend's 404. The rule stays
 * inverted here for the same reason a denylist is only ever as correct as
 * its last edit: a slug this function does not recognise as the app's own
 * is treated as a possible complex, never the other way round.
 *
 * `vercel.json`'s rewrite `source` already excludes these at the routing
 * layer; this list re-checks the same exclusion here so a direct hit on
 * `/api/prerender?slug=login` (bypassing that rewrite entirely) cannot be
 * proxied to the backend either.
 *
 * Derived from src/app/router/{authRoutes,ownerRoutes,adminRoutes}.tsx.
 */
const APP_ROUTES = new Set([
  'login',
  'register',
  'verify-email',
  'verify-email-sent',
  'forgot-password',
  'reset-password',
  'complexes',
  'onboarding',
  'settings',
  'dashboard',
  'bookings',
  'courts',
  'clients',
  'reports',
  'profile',
  'admin',
]);

/** The shape `slugify` produces and `complex.schemas.ts` accepts. */
const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/**
 * How long the prerender gets before the crawler is handed a 503 instead.
 *
 * Well under Googlebot's own patience: a slow answer that is still HTML
 * beats a timeout, but a crawl budget spent waiting on a cold backend is
 * worse than a retry it asks for itself.
 */
const PRERENDER_TIMEOUT_MS = 3000;
const RETRY_AFTER_SECONDS = 60;
const PRERENDER_RESULT_HEADER = 'x-prerender-result';
const VENUE_NOT_FOUND = 'venue-not-found';

/** Re-validates the slug the rewrite's `has`/`source` already gated on. */
function isValidSlug(slug: string | null): slug is string {
  if (!slug) return false;
  if (APP_ROUTES.has(slug)) return false;
  return SLUG_PATTERN.test(slug);
}

export async function GET(request: Request): Promise<Response> {
  const slug = new URL(request.url).searchParams.get('slug');
  if (!isValidSlug(slug)) return notFound();

  const backendUrl = process.env.BACKEND_URL ?? 'https://api.vibe.com.ar';

  try {
    const response = await fetch(`${backendUrl}/api/v1/public/prerender/${slug}`, {
      headers: { accept: 'text/html' },
      signal: AbortSignal.timeout(PRERENDER_TIMEOUT_MS),
    });

    // Only a 404 the backend explicitly marks as "this venue does not
    // exist" becomes a permanent 404. A bare 404 can be route-level (stale
    // BACKEND_URL, gateway, renamed path) and would de-index every venue.
    if (response.status === 404 && response.headers.get(PRERENDER_RESULT_HEADER) === VENUE_NOT_FOUND) {
      return notFound();
    }
    if (!response.ok) return retryLater();

    return new Response(response.body, {
      status: 200,
      headers: {
        'content-type': response.headers.get('content-type') ?? 'text/html; charset=utf-8',
        'cache-control': response.headers.get('cache-control') ?? 'public, max-age=300',
      },
    });
  } catch {
    return retryLater();
  }
}

function notFound(): Response {
  return new Response(null, { status: 404, headers: { 'cache-control': 'no-store' } });
}

/** 503 is the one failure a crawler retries instead of indexing or dropping the URL. */
function retryLater(): Response {
  return new Response(null, {
    status: 503,
    headers: { 'retry-after': String(RETRY_AFTER_SECONDS), 'cache-control': 'no-store' },
  });
}
