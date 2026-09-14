// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Google Identity Services in `ux_mode: 'redirect'` form-POSTs the credential
 * to `login_uri` — `/auth/google/callback` on the app's own origin, because
 * the `g_csrf_token` cookie it double-submits is set there. Nothing in the
 * bundle can observe what happens next: on Vercel that path is a rewrite to
 * the API, and if it is ever deleted or ordered after the SPA catch-all,
 * Google's POST is answered with `index.html` and every Google sign-in dies
 * silently. Hence this file, next to `vercel.headers.test.ts`.
 *
 * `vite.config.ts` carries the same hop for `pnpm dev` and the E2E stack; see
 * `vite.proxy.test.ts`.
 *
 * The crawler-prerender rewrite below replaced the former edge `middleware.ts`
 * (Vercel deprecated the edge runtime for Routing Middleware, and moving it to
 * `nodejs` would have run a function for every single-segment path, human
 * traffic included). A rewrite gated by `has: [{ type: 'header', key:
 * 'user-agent', ... }]` means only requests that already carry a matching
 * crawler user agent invoke any function at all.
 */

interface VercelHasCondition {
  type: string;
  key: string;
  value: string;
}

interface VercelRewrite {
  source: string;
  destination: string;
  has?: VercelHasCondition[];
}

const config = JSON.parse(readFileSync(fileURLToPath(new URL('./vercel.json', import.meta.url)), 'utf8')) as {
  rewrites: VercelRewrite[];
};

const GOOGLE_CALLBACK = '/auth/google/callback';
const SPA_CATCH_ALL = '/(.*)';

describe('vercel.json rewrites', () => {
  it('sends the Google redirect callback to the API', () => {
    const rewrite = config.rewrites.find((r) => r.source === GOOGLE_CALLBACK);
    expect(rewrite?.destination).toBe('https://api.vibe.com.ar/api/v1/auth/google/redirect');
  });

  // Vercel takes the first matching rewrite, and `/(.*)` matches everything.
  it('declares it before the SPA catch-all', () => {
    const callback = config.rewrites.findIndex((r) => r.source === GOOGLE_CALLBACK);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(callback).toBeGreaterThanOrEqual(0);
    expect(catchAll).toBeGreaterThanOrEqual(0);
    expect(callback).toBeLessThan(catchAll);
  });
});

/**
 * The routes the app owns at the root — ported from `APP_ROUTES` in the
 * former `middleware.ts` — so a crawler on any of them stays on the SPA
 * instead of being handed the backend's 404 for a slug that isn't one.
 *
 * Derived from src/app/router/{authRoutes,ownerRoutes,adminRoutes}.tsx.
 */
const APP_ROUTES = [
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
];

const PRERENDER_DESTINATION = '/api/prerender?slug=:slug';

/** Pulls the inner `path-to-regexp` group out of a `/:slug(...)` source. */
function slugGroup(source: string): string {
  const match = /^\/:slug\((.*)\)$/.exec(source);
  if (!match?.[1]) throw new Error(`not a /:slug(...) source: ${source}`);
  return match[1];
}

const crawlerRewrite = config.rewrites.find((r) => r.destination === PRERENDER_DESTINATION);

describe('vercel.json rewrites, crawler prerender', () => {
  it('exists, pointing at the api/prerender.ts Vercel Function', () => {
    expect(crawlerRewrite).toBeDefined();
  });

  it('is gated on the user-agent header', () => {
    expect(crawlerRewrite?.has).toHaveLength(1);
    expect(crawlerRewrite?.has?.[0]).toMatchObject({ type: 'header', key: 'user-agent' });
  });

  it('sits after the Google callback rewrite and before the SPA catch-all', () => {
    const callback = config.rewrites.findIndex((r) => r.source === GOOGLE_CALLBACK);
    const crawler = config.rewrites.findIndex((r) => r.destination === PRERENDER_DESTINATION);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(crawler).toBeGreaterThan(callback);
    expect(crawler).toBeLessThan(catchAll);
  });
});

if (!crawlerRewrite) throw new Error('crawler rewrite not found');
const slugPattern = new RegExp(`^${slugGroup(crawlerRewrite.source)}$`);

describe('vercel.json rewrites, crawler prerender: the slug shape', () => {
  it.each(APP_ROUTES)('rejects the app route %s', (route) => {
    expect(slugPattern.test(route)).toBe(false);
  });

  it.each(['club-padel-norte', 'demo'])('accepts the real slug %s', (slug) => {
    expect(slugPattern.test(slug)).toBe(true);
  });

  it('accepts login-norte: the exclusion is anchored to the whole segment', () => {
    expect(slugPattern.test('login-norte')).toBe(true);
  });

  it.each(['Foo_Bar', 'a.b', 'two/segments'])('rejects %s, which slugify could not have produced', (value) => {
    expect(slugPattern.test(value)).toBe(false);
  });
});

if (!crawlerRewrite.has?.[0]) throw new Error('crawler rewrite has no user-agent condition');
const uaPattern = new RegExp(crawlerRewrite.has[0].value);

const REAL_CRAWLER_USER_AGENTS: [string, string][] = [
  ['Googlebot desktop', 'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)'],
  [
    'Googlebot smartphone',
    'Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.101 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)',
  ],
  ['bingbot', 'Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)'],
  ['facebookexternalhit', 'facebookexternalhit/1.1'],
  ['WhatsApp', 'WhatsApp/2.23.20.0'],
  ['Twitterbot', 'Twitterbot/1.0'],
  ['LinkedInBot', 'LinkedInBot/1.0'],
  ['TelegramBot', 'TelegramBot (like TwitterBot)'],
  ['Discordbot', 'Discordbot/2.0'],
  ['Applebot', 'Applebot/0.1'],
  ['YandexBot', 'Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)'],
  ['Pinterestbot', 'Mozilla/5.0 (compatible; Pinterestbot/1.0; +http://www.pinterest.com/bot.html)'],
];

const REAL_BROWSER_USER_AGENTS: [string, string][] = [
  [
    'Chrome desktop',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36',
  ],
  [
    'Safari desktop',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15',
  ],
  ['Firefox desktop', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0'],
  [
    'Chrome mobile (Android)',
    'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36',
  ],
  [
    'Safari mobile (iOS)',
    'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
  ],
];

describe('vercel.json rewrites, crawler prerender: the user-agent condition', () => {
  it.each(REAL_CRAWLER_USER_AGENTS)('matches %s', (_name, userAgent) => {
    expect(uaPattern.test(userAgent)).toBe(true);
  });

  it.each(REAL_BROWSER_USER_AGENTS)('does not match %s', (_name, userAgent) => {
    expect(uaPattern.test(userAgent)).toBe(false);
  });
});
