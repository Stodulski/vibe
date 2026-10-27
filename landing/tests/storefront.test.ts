/**
 * The robots directive follows the one STOREFRONT_INDEXABLE switch, and error answers
 * stay out of the index whatever the switch says. Run with `pnpm test:unit`.
 */
import { createHash } from 'node:crypto';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import astroIslandScript from 'astro/runtime/server/astro-island.prebuilt.js';
import loadDirectiveScript from 'astro/runtime/client/load.prebuilt.js';
import onlyDirectiveScript from 'astro/runtime/client/only.prebuilt.js';
import {
  ASTRO_INLINE_SCRIPT_HASHES,
  robotsDirective,
  STOREFRONT_INDEXABLE,
  storefrontBookingConfig,
  storefrontRobots,
  STOREFRONT_CSP_REPORT_ONLY,
  STOREFRONT_REPORTING_ENDPOINTS,
} from '../src/lib/storefront.ts';

test('a successful answer follows the indexable switch', () => {
  assert.equal(robotsDirective(200), STOREFRONT_INDEXABLE ? 'index, follow' : 'noindex');
});

test('error answers are never indexable', () => {
  assert.equal(robotsDirective(404), 'noindex');
  assert.equal(robotsDirective(503), 'noindex');
});

test('booking steps are never indexable, whatever the switch says', () => {
  for (const path of ['/c/club-sur/book', '/c/club-sur/book/confirm', '/c/club-sur/book/success', '/c/club-sur/book/cancel']) {
    assert.equal(storefrontRobots(path, 200), 'noindex', path);
  }
});

test('the complex page follows the switch and the status', () => {
  assert.equal(storefrontRobots('/c/club-sur', 200), robotsDirective(200));
  assert.equal(storefrontRobots('/c/club-sur', 404), 'noindex');
});

test('a deleted complex (410) follows the switch like a live page, and is never noindex of its own', () => {
  assert.equal(robotsDirective(410), robotsDirective(200));
  assert.equal(storefrontRobots('/c/club-viejo', 410), robotsDirective(200));
  if (STOREFRONT_INDEXABLE) assert.notEqual(storefrontRobots('/c/club-viejo', 410), 'noindex');
});

test('a slug that only starts with book is not a booking step', () => {
  assert.equal(storefrontRobots('/c/bookshop', 200), robotsDirective(200));
});

test('the booking config points at the storefront origin and its privacy page', () => {
  assert.deepEqual(storefrontBookingConfig('https://api.vibe.com.ar/api/v1'), {
    apiBaseUrl: 'https://api.vibe.com.ar/api/v1',
    publicSiteUrl: 'https://vibe.com.ar',
    privacyUrl: 'https://vibe.com.ar/privacidad',
  });
});

test('the storefront CSP is report-only policy that keeps the directives the flow needs', () => {
  const directives = STOREFRONT_CSP_REPORT_ONLY.split('; ');
  assert.ok(directives.includes("frame-ancestors 'none'"));
  assert.ok(directives.includes("connect-src 'self' https://api.vibe.com.ar https://vibe.com.ar https://cloudflareinsights.com"));
  assert.ok(directives.includes(`script-src 'self' ${ASTRO_INLINE_SCRIPT_HASHES.join(' ')} https://static.cloudflareinsights.com`));
  assert.ok(directives.some((d) => d.startsWith('img-src ') && d.includes('https://*.tile.openstreetmap.org')));
});

test('the storefront CSP reports to the same-origin /_r/s endpoint, as the app does', () => {
  const directives = STOREFRONT_CSP_REPORT_ONLY.split('; ');
  assert.ok(directives.includes('report-uri /_r/s'));
  assert.ok(directives.includes('report-to csp-endpoint'));
  assert.equal(STOREFRONT_REPORTING_ENDPOINTS, 'csp-endpoint="https://vibe.com.ar/_r/s"');
});

test('the storefront CSP names no Turnstile host (the booking flow loads none)', () => {
  assert.ok(!STOREFRONT_CSP_REPORT_ONLY.includes('challenges.cloudflare.com'));
});

// The islands on /c/<slug> need Astro's inline scripts. Hashed from Astro's own copies,
// so an upgrade that changes them fails here instead of in the browser.
test('the storefront CSP allows the inline scripts Astro writes for its islands', () => {
  const sha256 = (script: string) => `'sha256-${createHash('sha256').update(script).digest('base64')}'`;
  const scriptSrc = STOREFRONT_CSP_REPORT_ONLY.split('; ').find((d) => d.startsWith('script-src '));
  for (const script of [loadDirectiveScript, onlyDirectiveScript, astroIslandScript]) {
    assert.ok(scriptSrc?.split(' ').includes(sha256(script)), `script-src is missing ${sha256(script)}`);
  }
  assert.ok(!scriptSrc?.includes("'unsafe-inline'"));
});
