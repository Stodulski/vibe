/**
 * The robots directive follows the one STOREFRONT_INDEXABLE switch, and error answers
 * stay out of the index whatever the switch says. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { robotsDirective, STOREFRONT_INDEXABLE } from '../src/lib/storefront.ts';

test('a successful answer follows the indexable switch', () => {
  assert.equal(robotsDirective(200), STOREFRONT_INDEXABLE ? 'index, follow' : 'noindex');
});

test('error answers are never indexable', () => {
  assert.equal(robotsDirective(404), 'noindex');
  assert.equal(robotsDirective(503), 'noindex');
});
