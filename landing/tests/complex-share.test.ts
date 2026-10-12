/**
 * The share image of a complex: which facts it draws, the cache-busting version and the URL
 * the complex page points at. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  activeSportLabels,
  complexShareFacts,
  shareImagePath,
  shareImageUrl,
  shareImageVersion,
} from '../src/lib/complex-share.ts';

const facts = {
  name: 'La Zona F5',
  city: 'Caleta Olivia',
  province: 'Santa Cruz',
  sports: ['Pádel', 'Fútbol'],
};

test('the version is 8 hex characters and stays the same for the same facts', () => {
  const version = shareImageVersion(facts);
  assert.match(version, /^[0-9a-f]{8}$/);
  assert.equal(shareImageVersion({ ...facts, sports: [...facts.sports] }), version);
});

test('the version changes when the name, city, province or sports change', () => {
  const version = shareImageVersion(facts);
  assert.notEqual(shareImageVersion({ ...facts, name: 'La Zona F6' }), version);
  assert.notEqual(shareImageVersion({ ...facts, city: 'Puerto Deseado' }), version);
  assert.notEqual(shareImageVersion({ ...facts, province: 'Chubut' }), version);
  assert.notEqual(shareImageVersion({ ...facts, sports: ['Pádel'] }), version);
  assert.notEqual(shareImageVersion({ ...facts, sports: ['Pádel', 'Tenis'] }), version);
});

test('the version does not mix fields: moving text between name and city changes it', () => {
  const split = shareImageVersion({ name: 'Club A', city: 'B', province: '', sports: [] });
  const joined = shareImageVersion({ name: 'Club', city: 'A B', province: '', sports: [] });
  assert.notEqual(split, joined);
});

test('the version ignores spacing that the picture does not show', () => {
  assert.equal(
    shareImageVersion({ ...facts, name: '  La   Zona F5 ' }),
    shareImageVersion(facts),
  );
});

test('the image path carries the slug and the version, with the image extension', () => {
  const path = shareImagePath('la-zona-f5', facts);
  assert.equal(path, `/og/c/la-zona-f5.png?v=${shareImageVersion(facts)}`);
});

test('the absolute URL is the storefront origin plus the image path', () => {
  assert.equal(shareImageUrl('la-zona-f5', facts), `https://vibe.com.ar${shareImagePath('la-zona-f5', facts)}`);
  assert.equal(shareImageUrl('a b', facts).startsWith('https://vibe.com.ar/og/c/a%20b.png?v='), true);
});

test('active sport labels keep the active courts only, unique and in order', () => {
  const labels = { padel: 'Pádel', soccer: 'Fútbol', tennis: 'Tenis' };
  const courts = [
    { sport: 'soccer', is_active: true },
    { sport: 'padel', is_active: true },
    { sport: 'padel', is_active: true },
    { sport: 'tennis', is_active: false },
    { sport: 'unknown', is_active: true },
  ];
  assert.deepEqual(activeSportLabels(courts, labels), ['Fútbol', 'Pádel']);
});

test('complex share facts read the name, city, province and active sports of the payload', () => {
  const labels = { padel: 'Pádel', soccer: 'Fútbol' };
  const payload = {
    complex: { name: 'La Zona F5', city: 'Caleta Olivia', province: 'Santa Cruz', payments_enabled: false },
    courts: [{ sport: 'soccer', is_active: true }],
  };
  assert.deepEqual(complexShareFacts(payload, labels), {
    name: 'La Zona F5',
    city: 'Caleta Olivia',
    province: 'Santa Cruz',
    sports: ['Fútbol'],
  });
});
