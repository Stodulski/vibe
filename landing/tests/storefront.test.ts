/**
 * The robots directive follows the one STOREFRONT_INDEXABLE switch, and error answers
 * stay out of the index whatever the switch says. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  robotsDirective,
  STOREFRONT_INDEXABLE,
  storefrontBookingConfig,
  storefrontRobots,
  STOREFRONT_CSP_REPORT_ONLY,
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
  assert.ok(directives.includes("connect-src 'self' https://api.vibe.com.ar https://vibe.com.ar"));
  assert.ok(directives.some((d) => d.startsWith('img-src ') && d.includes('https://*.tile.openstreetmap.org')));
});

test('the storefront CSP names no report endpoint and no Turnstile host', () => {
  assert.ok(!STOREFRONT_CSP_REPORT_ONLY.includes('report-uri'));
  assert.ok(!STOREFRONT_CSP_REPORT_ONLY.includes('report-to'));
  assert.ok(!STOREFRONT_CSP_REPORT_ONLY.includes('challenges.cloudflare.com'));
});
