/**
 * robots.txt: the base is served byte for byte, and the storefront sitemap line is added
 * only while STOREFRONT_INDEXABLE is on. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { robotsTxt } from '../src/lib/robots.ts';
import { STOREFRONT_INDEXABLE } from '../src/lib/storefront.ts';

const base = readFileSync(new URL('../src/lib/robots-base.txt', import.meta.url), 'utf8');
const storefrontLine = 'Sitemap: https://vibe.com.ar/sitemap-storefront.xml\n';

test('the base keeps the main sitemap as its last line', () => {
  assert.ok(base.endsWith('Sitemap: https://vibe.com.ar/sitemap.xml\n'));
});

test('with the switch off the output is the base file, byte for byte', () => {
  assert.equal(robotsTxt(base, false), base);
});

test('with the switch on the storefront sitemap follows the main one and nothing else changes', () => {
  const out = robotsTxt(base, true);
  assert.equal(out, base + storefrontLine);
  assert.ok(out.startsWith(base));
});

test('the build output matches the switch', () => {
  assert.equal(robotsTxt(base, STOREFRONT_INDEXABLE), STOREFRONT_INDEXABLE ? base + storefrontLine : base);
});
