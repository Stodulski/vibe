// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * This app serves no public pages. Complex storefronts, the booking flow and the
 * city hubs live on vibe.com.ar (the `landing/` package), so this file forwards
 * nothing to the API for them: no crawler prerender, no hub, no sitemap. Every
 * path the app does not own falls through to the SPA catch-all, which answers
 * with the 404 page.
 *
 * `/auth/google/callback` is an ordinary client-side route (`GoogleCallbackPage`,
 * see `authRoutes.tsx`): Google lands the browser there with `?code=&state=`,
 * and the SPA catch-all serves it `index.html` like every other route.
 */

interface VercelRewrite {
  source: string;
  destination: string;
  has?: { type: string; key: string; value: string }[];
}

const config = JSON.parse(readFileSync(fileURLToPath(new URL('./vercel.json', import.meta.url)), 'utf8')) as {
  rewrites: VercelRewrite[];
};

const GOOGLE_CALLBACK = '/auth/google/callback';
const SPA_CATCH_ALL = '/(.*)';
const TUNNEL_ENVELOPE = '/_r/e';
const TUNNEL_SECURITY = '/_r/s';

/**
 * `Sentry.init({ tunnel: '/_r/e' })` (src/shared/lib/sentry.ts) and the CSP's
 * `report-uri /_r/s` (vercel.json headers) both point at this app's own
 * origin instead of straight at ingest.us.sentry.io, because ad blockers
 * strip requests to that host by name (`ERR_BLOCKED_BY_CLIENT`). These two
 * rewrites are what actually forwards the traffic to Sentry.
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

  it('declares both tunnels before the SPA catch-all', () => {
    const envelope = config.rewrites.findIndex((r) => r.source === TUNNEL_ENVELOPE);
    const security = config.rewrites.findIndex((r) => r.source === TUNNEL_SECURITY);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(envelope).toBeGreaterThanOrEqual(0);
    expect(security).toBeGreaterThanOrEqual(0);
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

describe('vercel.json rewrites, SPA fallback', () => {
  // The OIDC callback is a plain SPA route, not a rewrite target. This guards
  // against a future rewrite reappearing ahead of the catch-all and silently
  // swallowing the page again.
  it('has no dedicated rewrite for the Google OIDC callback: it falls through to the SPA catch-all', () => {
    const rewrite = config.rewrites.find((r) => r.source === GOOGLE_CALLBACK);
    expect(rewrite).toBeUndefined();
  });

  it('ends with the SPA catch-all, so every unknown path is answered by the router', () => {
    expect(config.rewrites.at(-1)?.source).toBe(SPA_CATCH_ALL);
    expect(config.rewrites.at(-1)?.destination).toBe('/index.html');
  });
});

describe('vercel.json rewrites, no public pages on the API', () => {
  it('forwards nothing to the API origin: no hub, sitemap or storefront is served from the app', () => {
    const toApi = config.rewrites.filter((r) => r.destination.includes('api.vibe.com.ar'));
    expect(toApi).toEqual([]);
  });

  it('has no crawler prerender rewrite left, gated on the user agent or not', () => {
    const prerender = config.rewrites.filter((r) => r.destination.startsWith('/api/'));
    expect(prerender).toEqual([]);
  });

  it.each(['/c/:slug([a-z0-9]+(?:-[a-z0-9]+)*)', '/canchas/:city', '/sitemap.xml'])(
    'no longer rewrites %s',
    (source) => {
      expect(config.rewrites.find((r) => r.source === source)).toBeUndefined();
    },
  );

  it('has no rewrite that depends on the user agent', () => {
    expect(config.rewrites.filter((r) => r.has !== undefined)).toEqual([]);
  });
});
