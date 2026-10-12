/**
 * The complex page (/c/<slug>) is the club's own storefront. It sends no player to another club:
 * no link to the city hub, and no BreadcrumbList that would name the city's other clubs as the way
 * back. The page is read as source, the same way negotiate.test.ts reads the middleware.
 * Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const page = readFileSync(new URL('../src/pages/c/[slug]/index.astro', import.meta.url), 'utf8');

test('the complex page carries no BreadcrumbList and no breadcrumb helper', () => {
  assert.equal(page.includes('BreadcrumbList'), false);
  assert.equal(page.includes('breadcrumbJsonLd'), false);
});

test('the complex page links to no city hub', () => {
  assert.equal(page.includes('Más canchas'), false);
  assert.equal(page.includes('nearbyLink'), false);
  assert.equal(page.includes('cityPath'), false);
});

test('the complex page keeps its own complex JSON-LD', () => {
  assert.ok(page.includes('buildComplexSchema('));
});
