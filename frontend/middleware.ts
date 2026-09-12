import { next } from '@vercel/edge';

// Bots that need server-rendered HTML for meta tags / OG previews.
const BOT_PATTERN =
  /bot|crawl|spider|slurp|facebookexternalhit|WhatsApp|Twitterbot|LinkedInBot|Discordbot|TelegramBot|Googlebot|bingbot|yandex|Applebot|Pinterestbot/i;

/**
 * The single-segment paths the app owns at the root.
 *
 * This used to be the other way round: a SKIP_PREFIXES denylist that anything
 * unlisted fell through. It drifted — /profile, /reports and /admin were added
 * to the router and never added here, so a crawler on /profile was rewritten to
 * the prerender endpoint and served the backend's 404. A denylist is only ever
 * as correct as its last edit.
 *
 * So the rule is inverted: nothing is prerendered unless it looks like a slug
 * and is not one of these. A new root route that is forgotten here now fails
 * closed — it is served by the SPA, which is what it wanted anyway.
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
 * How long the prerender gets before the crawler is handed the SPA shell.
 *
 * Well under Googlebot's own patience: a slow answer that is still HTML beats a
 * timeout, but a crawl budget spent waiting on a cold backend is worse than an
 * empty shell it can render itself.
 */
const PRERENDER_TIMEOUT_MS = 3000;

/**
 * The slug this path is asking for, or null if the SPA should serve it.
 *
 * A slug is exactly one segment, has no extension (so /robots.txt, /sw.js and
 * /manifest.json stay files), is not a route the app owns, and matches the
 * shape the complex form generates.
 */
function slugOf(pathname: string): string | null {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length !== 1) return null;

  const segment = segments[0];
  if (!segment) return null;
  if (segment.includes('.')) return null;
  if (APP_ROUTES.has(segment)) return null;
  if (!SLUG_PATTERN.test(segment)) return null;

  return segment;
}

export default async function middleware(request: Request): Promise<Response> {
  const slug = slugOf(new URL(request.url).pathname);
  if (slug === null) return next();

  const userAgent = request.headers.get('user-agent') ?? '';
  if (!BOT_PATTERN.test(userAgent)) return next();

  // Fetched rather than rewritten so the answer can be inspected. A rewrite
  // hands the crawler whatever the backend says, including the 500s it still
  // returns when the complex lookup or the template fetch fails — and a 500 on
  // a public page is a URL a crawler drops. The shell is always a valid answer:
  // slower to index, never an error.
  const apiUrl = process.env.BACKEND_URL ?? 'https://api.vibe.com.ar';
  try {
    const response = await fetch(`${apiUrl}/api/v1/public/prerender/${slug}`, {
      headers: { accept: 'text/html' },
      signal: AbortSignal.timeout(PRERENDER_TIMEOUT_MS),
    });
    if (!response.ok) return next();

    return new Response(response.body, {
      status: 200,
      headers: {
        'content-type': response.headers.get('content-type') ?? 'text/html; charset=utf-8',
        'cache-control': response.headers.get('cache-control') ?? 'public, max-age=300',
      },
    });
  } catch {
    // Timed out, DNS, TLS, connection refused. The crawler gets the shell.
    return next();
  }
}

export const config = {
  // One segment only. The old matcher ran on every request just to call next()
  // on almost all of them.
  matcher: '/:slug',
};
