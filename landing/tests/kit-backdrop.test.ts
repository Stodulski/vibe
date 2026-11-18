import { test } from 'node:test';
import assert from 'node:assert/strict';
import { kitBackdrop } from '../src/lib/kit/backdrop.ts';

test('the first sport with a photo picks the backdrop', () => {
  assert.equal(kitBackdrop(['Pickleball', 'Fútbol', 'Pádel']), 'futbol');
  assert.equal(kitBackdrop(['Tenis']), 'tenis');
  assert.equal(kitBackdrop(['Pádel', 'Tenis']), 'padel');
});

test('a complex with no sport that has a photo gets the padel photo', () => {
  assert.equal(kitBackdrop([]), 'padel');
  assert.equal(kitBackdrop(['Vóley', 'Hockey']), 'padel');
});
