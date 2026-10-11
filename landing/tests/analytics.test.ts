/**
 * The Cloudflare Web Analytics beacon: no script without a token, and the exact token
 * in the attribute when there is one. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { cfBeaconAttribute, CF_BEACON_SRC } from '../src/lib/analytics.ts';

test('no token means no beacon attribute, so no script is rendered', () => {
  assert.equal(cfBeaconAttribute(undefined), null);
  assert.equal(cfBeaconAttribute(null), null);
  assert.equal(cfBeaconAttribute(''), null);
  assert.equal(cfBeaconAttribute('   '), null);
});

test('a token becomes the documented attribute value, exactly', () => {
  assert.equal(cfBeaconAttribute('0123abcd4567ef89'), '{"token": "0123abcd4567ef89"}');
});

test('surrounding whitespace is not part of the token', () => {
  assert.equal(cfBeaconAttribute(' tok-en_1 \n'), '{"token": "tok-en_1"}');
});

test('a value that could break out of the attribute is refused', () => {
  assert.equal(cfBeaconAttribute('abc"><script>'), null);
  assert.equal(cfBeaconAttribute("abc' onload='x"), null);
});

test('the beacon script is Cloudflare\'s standard URL', () => {
  assert.equal(CF_BEACON_SRC, 'https://static.cloudflareinsights.com/beacon.min.js');
});
