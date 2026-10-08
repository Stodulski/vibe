// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * `/auth/google/callback` is an ordinary client-side route (`GoogleCallbackPage`,
 * see `authRoutes.tsx`) since the OIDC authorization-code flow replaced
 * Google Identity Services' redirect mode: Google's authorization server
 * lands the browser there with `?code=&state=`, and the SPA catch-all below
 * is what serves it `index.html` like every other route. There is no
 * Vercel-side rewrite for it any more — that only existed to hand Google's
 * old form POST to the API — so this file no longer pins one; see
 * `vercel.json`'s git history for the removed entry.
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

interface VercelHeaderRule {
  source: string;
  headers: { key: string; value: string }[];
}

const config = JSON.parse(readFileSync(fileURLToPath(new URL('./vercel.json', import.meta.url)), 'utf8')) as {
  rewrites: VercelRewrite[];
  headers: VercelHeaderRule[];
};

const GOOGLE_CALLBACK = '/auth/google/callback';
const SPA_CATCH_ALL = '/(.*)';
const TUNNEL_ENVELOPE = '/_r/e';
const TUNNEL_SECURITY = '/_r/s';

/**
 * `Sentry.init({ tunnel: '/_r/e' })` (src/shared/lib/sentry.ts) and the CSP's
 * `report-uri /_r/s` (vercel.json headers) both point at this app's own
 * origin instead of straight at ingest.us.sentry.io, because ad blockers
 * strip requests to that host by name (`ERR_BLOCKED_BY_CLIENT`) — these two
 * rewrites are what actually forwards the traffic to Sentry. Neither path
 * sits under the `/c/` prefix the crawler rewrite below is scoped to.
 */
describe('vercel.json rewrites, Sentry tunnel', () => {
  it('rewrites the envelope tunnel to the Sentry envelope endpoint', () => {
    const rewrite = config.rewrites.find((r) => r.source === TUNNEL_ENVELOPE);
    expect(rewrite?.destination).toBe(
      'https://o4511023559868416.ingest.us.sentry.io/api/4512086199631872/envelope/?sentry_key=a544aae361b0942642803f08c82eea60',
    );
  });

  it('rewrites the CSP report tunnel to the Sentry security endpoint', () => {
    const rewrite = config.rewrites.find((r) => r.source === TUNNEL_SECURITY);
    expect(rewrite?.destination).toBe(
      'https://o4511023559868416.ingest.us.sentry.io/api/4512086199631872/security/?sentry_key=a544aae361b0942642803f08c82eea60',
    );
  });

  it('declares both tunnels before the crawler rewrite and the SPA catch-all', () => {
    const envelope = config.rewrites.findIndex((r) => r.source === TUNNEL_ENVELOPE);
    const security = config.rewrites.findIndex((r) => r.source === TUNNEL_SECURITY);
    const crawler = config.rewrites.findIndex((r) => r.destination === '/api/prerender?slug=:slug');
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(envelope).toBeGreaterThanOrEqual(0);
    expect(security).toBeGreaterThanOrEqual(0);
    expect(envelope).toBeLessThan(crawler);
    expect(security).toBeLessThan(crawler);
    expect(envelope).toBeLessThan(catchAll);
    expect(security).toBeLessThan(catchAll);
  });

  it('points both tunnels at the same Sentry project, with a matching sentry_key', () => {
    const envelope = config.rewrites.find((r) => r.source === TUNNEL_ENVELOPE);
    const security = config.rewrites.find((r) => r.source === TUNNEL_SECURITY);
    const projectPattern = /^https:\/\/o4511023559868416\.ingest\.us\.sentry\.io\/api\/4512086199631872\//;

    expect(envelope?.destination).toMatch(projectPattern);
    expect(security?.destination).toMatch(projectPattern);

    const keyPattern = /sentry_key=([a-f0-9]{32})/;
    const envelopeKey = keyPattern.exec(envelope?.destination ?? '')?.[1];
    const securityKey = keyPattern.exec(security?.destination ?? '')?.[1];

    expect(envelopeKey).toMatch(/^[a-f0-9]{32}$/);
    expect(envelopeKey).toBe(securityKey);
  });
});

describe('vercel.json rewrites', () => {
  // The OIDC callback is a plain SPA route now, not a rewrite target — this
  // guards against a future rewrite reappearing ahead of the catch-all and
  // silently swallowing the page again.
  it('has no dedicated rewrite for the Google OIDC callback: it falls through to the SPA catch-all', () => {
    const rewrite = config.rewrites.find((r) => r.source === GOOGLE_CALLBACK);
    expect(rewrite).toBeUndefined();
  });
});

const PRERENDER_DESTINATION = '/api/prerender?slug=:slug';

/**
 * Complex storefronts live under `/c/<slug>`, so the crawler rewrite is scoped
 * to that prefix. A single-segment `/<slug>` is no longer a complex URL at all:
 * it is the legacy redirect the SPA answers for humans, and crawlers get the
 * plain shell for it (the old URLs are not in use, so they are not rewritten).
 */
const crawlerRewrites = config.rewrites.filter((r) => r.destination === PRERENDER_DESTINATION);
const crawlerRewrite = crawlerRewrites[0];

/** Pulls the inner `path-to-regexp` group out of a `/c/:slug(...)` source. */
function slugGroup(source: string): string {
  const match = /^\/c\/:slug\((.*)\)$/.exec(source);
  if (!match?.[1]) throw new Error(`not a /c/:slug(...) source: ${source}`);
  return match[1];
}

describe('vercel.json rewrites, crawler prerender', () => {
  it('exists, pointing at the api/prerender.ts Vercel Function', () => {
    expect(crawlerRewrite).toBeDefined();
  });

  it('is scoped to the /c/:slug prefix', () => {
    expect(crawlerRewrite?.source).toMatch(/^\/c\/:slug\(.+\)$/);
  });

  it('has no single-segment /:slug crawler rule left over', () => {
    expect(crawlerRewrites).toHaveLength(1);
    const singleSegment = config.rewrites.filter((r) => r.source.startsWith('/:slug('));
    expect(singleSegment).toEqual([]);
  });

  it('is gated on the user-agent header', () => {
    expect(crawlerRewrite?.has).toHaveLength(1);
    expect(crawlerRewrite?.has?.[0]).toMatchObject({ type: 'header', key: 'user-agent' });
  });

  it('sits before the SPA catch-all', () => {
    const crawler = config.rewrites.findIndex((r) => r.destination === PRERENDER_DESTINATION);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(crawler).toBeLessThan(catchAll);
  });
});

if (!crawlerRewrite) throw new Error('crawler rewrite not found');
const crawlerPath = new RegExp(`^/c/(?:${slugGroup(crawlerRewrite.source)})$`);

describe('vercel.json rewrites, crawler prerender: the path shape', () => {
  it.each(['/c/club-padel-norte', '/c/demo', '/c/login-norte'])('matches the complex page %s', (path) => {
    expect(crawlerPath.test(path)).toBe(true);
  });

  it.each([
    '/club-padel-norte',
    '/login',
    '/c',
    '/c/',
    '/c/Foo_Bar',
    '/c/a.b',
    '/c/two/segments',
    '/c/club-padel-norte/book',
  ])('does not match %s', (path) => {
    expect(crawlerPath.test(path)).toBe(false);
  });
});

describe('vercel.json rewrites, Sentry tunnel: never shadowed by the crawler rule', () => {
  it.each([TUNNEL_ENVELOPE, TUNNEL_SECURITY])('%s does not match the crawler path', (source) => {
    expect(crawlerPath.test(source)).toBe(false);
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

/**
 * Booking pages are per-complex and must stay out of search results. The
 * noindex rules follow the `/c/:slug` prefix, so the old `/:slug/book` rules
 * must be gone or they would also match the platform's own two-segment paths.
 */
describe('vercel.json headers, noindex on public booking pages', () => {
  const noindexSources = config.headers
    .filter((rule) => rule.headers.some((h) => h.key === 'X-Robots-Tag' && h.value === 'noindex'))
    .map((rule) => rule.source);

  it.each(['/c/:slug/book', '/c/:slug/book/(.*)'])('marks %s noindex', (source) => {
    expect(noindexSources).toContain(source);
  });

  it.each(['/:slug/book', '/:slug/book/(.*)'])('no longer carries the old %s noindex rule', (source) => {
    expect(noindexSources).not.toContain(source);
  });
});

/**
 * `/canchas/:city` is the public hub listing the complexes of one city. It is
 * server-rendered by the backend (`GET /api/v1/public/hubs/:city`), so it is
 * forwarded to the API origin the same way `/sitemap.xml` is. It is not gated
 * on the user agent: people and crawlers get the same HTML.
 */
const HUB_SOURCE = '/canchas/:city';
const HUB_DESTINATION = 'https://api.vibe.com.ar/api/v1/public/hubs/:city';

describe('vercel.json rewrites, city hubs', () => {
  const hub = config.rewrites.find((r) => r.source === HUB_SOURCE);

  it('rewrites /canchas/:city to the backend city hub endpoint', () => {
    expect(hub?.destination).toBe(HUB_DESTINATION);
  });

  it('is not gated on the user agent, so people and crawlers get the same page', () => {
    expect(hub?.has).toBeUndefined();
  });

  it('sits before the SPA catch-all', () => {
    const hubIndex = config.rewrites.findIndex((r) => r.source === HUB_SOURCE);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(hubIndex).toBeGreaterThanOrEqual(0);
    expect(hubIndex).toBeLessThan(catchAll);
  });
});
