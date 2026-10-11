/**
 * /404 and /404.html are answered by the Vercel middleware with a rewrite to NOT_FOUND_TARGET, a
 * path with no file and no matcher line, so Vercel serves its 404 and the rewrite cannot loop.
 * Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { config } from '../middleware.ts';
import { matcherCovers, notFoundTarget, NOT_FOUND_TARGET } from '../src/lib/markdown/negotiate.ts';

test('the 404 names are rewritten to the not-found target, and no other path is', () => {
  assert.equal(notFoundTarget('/404'), NOT_FOUND_TARGET);
  assert.equal(notFoundTarget('/404/'), NOT_FOUND_TARGET);
  assert.equal(notFoundTarget('/404.html'), NOT_FOUND_TARGET);
  for (const path of ['/', '/guias', '/guias/404', '/404-algo', '/404.htm', '/canchas/404']) {
    assert.equal(notFoundTarget(path), null, path);
  }
});

test('the middleware runs on the 404 names, so they are answered before the static file', () => {
  assert.ok(config.matcher.includes('/404'));
  assert.ok(config.matcher.includes('/404.html'));
});

test('the rewrite target runs no middleware, so the rewrite cannot loop', () => {
  assert.equal(config.matcher.includes(NOT_FOUND_TARGET), false);
  assert.equal(matcherCovers(NOT_FOUND_TARGET), false);
});

test('no page answers the not-found target, so Vercel falls through to its 404', () => {
  const pages = join(fileURLToPath(new URL('..', import.meta.url)), 'src', 'pages');
  const root = readdirSync(pages);
  assert.equal(root.includes(`${NOT_FOUND_TARGET.slice(1)}.astro`), false);
  // A root catch-all such as [slug].astro would answer any single-segment path.
  assert.equal(root.some((name) => name.startsWith('[')), false);
});
