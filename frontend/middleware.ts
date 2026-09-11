import { next, rewrite } from '@vercel/edge';

// Bots that need server-rendered HTML for meta tags / OG previews.
const BOT_PATTERN =
  /bot|crawl|spider|slurp|facebookexternalhit|WhatsApp|Twitterbot|LinkedInBot|Discordbot|TelegramBot|Googlebot|bingbot|yandex|Applebot|Pinterestbot/i;

// Only intercept /:slug routes (public complex pages).
// Skip known frontend paths and static assets.
const SKIP_PREFIXES = [
  '/login',
  '/register',
  '/verify-email',
  '/forgot-password',
  '/reset-password',
  '/complexes',
  '/onboarding',
  '/settings',
  '/dashboard',
  '/bookings',
  '/courts',
  '/clients',
  '/assets/',
  '/fonts/',
  '/icons/',
  '/api/',
  '/sw.js',
  '/manifest.json',
  '/logo',
];

export default function middleware(request: Request) {
  const url = new URL(request.url);
  const path = url.pathname;

  // Skip non-slug routes.
  if (path === '/' || SKIP_PREFIXES.some((p) => path.startsWith(p))) {
    return next();
  }

  // Skip paths with more than one segment that aren't /:slug/book/* patterns.
  // We only prerender the main /:slug page.
  const segments = path.split('/').filter(Boolean);
  if (segments.length !== 1) {
    return next();
  }

  const userAgent = request.headers.get('user-agent') ?? '';
  if (!BOT_PATTERN.test(userAgent)) {
    return next();
  }

  // Bot detected on a /:slug route — rewrite to the backend prerender endpoint.
  const slug = segments[0];
  if (!slug) return next();
  const apiUrl = process.env.BACKEND_URL ?? 'https://api.vibe.com.ar';
  return rewrite(`${apiUrl}/api/v1/public/prerender/${slug}`);
}

export const config = {
  matcher: '/((?!_next/|_vercel/).*)',
};
