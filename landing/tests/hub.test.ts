/**
 * Unit tests for the hub view model. Run with `pnpm test:unit`.
 * The expected copy is the one backend/internal/publicsite/hub.go renders.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  HUB_DESCRIPTION_MAX,
  hubDescription,
  jsonLdScript,
  parseHubData,
  sportLabel,
  toHubView,
  type HubData,
} from '../src/lib/hub.ts';

const banfield: HubData = {
  hub: { slug: 'banfield', name: 'Banfield' },
  complexes: [
    { slug: 'club-sur', name: 'Club Sur', address: 'Av. Siempre Viva 742', sports: ['padel', 'tennis'] },
  ],
};

test('one name is named alone', () => {
  assert.equal(hubDescription('Banfield', ['Club Sur']), 'Reservá tu cancha en Banfield: Club Sur.');
});

test('two names are joined with "y"', () => {
  assert.equal(hubDescription('Banfield', ['A', 'B']), 'Reservá tu cancha en Banfield: A y B.');
});

test('three names are joined with commas and "y"', () => {
  assert.equal(hubDescription('Banfield', ['A', 'B', 'C']), 'Reservá tu cancha en Banfield: A, B y C.');
});

test('more than three names name the first three and count the rest', () => {
  assert.equal(
    hubDescription('Banfield', ['A', 'B', 'C', 'D', 'E']),
    'Reservá tu cancha en Banfield: A, B, C y 2 más.',
  );
});

test('a long description is cut to the limit and ends with an ellipsis', () => {
  const names = Array.from({ length: 3 }, (_, i) => `Complejo ${'x'.repeat(60)} ${i}`);
  const description = hubDescription('Banfield', names);
  const runes = Array.from(description);

  assert.equal(runes.length, HUB_DESCRIPTION_MAX);
  assert.ok(description.endsWith('...'));
});

test('a cut never splits a multi-byte letter', () => {
  const names = [`Ñandú ${'ñ'.repeat(200)}`];
  const description = hubDescription('Banfield', names);

  assert.equal(Array.from(description).length, HUB_DESCRIPTION_MAX);
  assert.ok(!description.includes('�'));
});

test('known sports get their Spanish label', () => {
  assert.equal(sportLabel('padel'), 'Pádel');
  assert.equal(sportLabel('basketball'), 'Básquet');
});

test('unknown sports are shown as stored, including prototype names', () => {
  assert.equal(sportLabel('squash'), 'squash');
  assert.equal(sportLabel('constructor'), 'constructor');
});

test('the view carries the hub title, H1 and canonical of the backend page', () => {
  const view = toHubView(banfield);

  assert.equal(view.title, 'Canchas en Banfield - Reservá tu cancha | Vibe');
  assert.equal(view.h1, 'Canchas en Banfield');
  assert.equal(view.canonical, 'https://vibe.com.ar/canchas/banfield');
  assert.equal(view.description, 'Reservá tu cancha en Banfield: Club Sur.');
});

test('complex links are relative in the list and labelled sports are shown', () => {
  const [complex] = toHubView(banfield).complexes;

  assert.equal(complex.href, '/c/club-sur');
  assert.equal(complex.address, 'Av. Siempre Viva 742');
  assert.deepEqual(complex.sports, ['Pádel', 'Tenis']);
});

test('the ItemList JSON-LD uses absolute complex URLs in order', () => {
  const list = JSON.parse(toHubView(banfield).jsonLd);

  assert.equal(list['@context'], 'https://schema.org');
  assert.equal(list['@type'], 'ItemList');
  assert.equal(list.name, 'Canchas en Banfield');
  assert.deepEqual(list.itemListElement, [
    { '@type': 'ListItem', position: 1, url: 'https://vibe.com.ar/c/club-sur' },
  ]);
});

test('JSON-LD escapes characters that could close the script element', () => {
  const hostile = toHubView({
    hub: { slug: 'banfield', name: '</script><b>&' },
    complexes: [],
  });

  assert.ok(!/[<>&]/.test(hostile.jsonLd));
  assert.equal(JSON.parse(hostile.jsonLd).name, 'Canchas en </script><b>&');
});

test('jsonLdScript round-trips line separators as valid JSON', () => {
  const value = { text: 'a\u2028b\u2029c' };
  assert.deepEqual(JSON.parse(jsonLdScript(value)), value);
});

test('parseHubData reads the fields the page uses', () => {
  const parsed = parseHubData({
    hub: { slug: 'banfield', name: 'Banfield', complex_count: 1 },
    complexes: [{ slug: 'club-sur', name: 'Club Sur', address: 'Av. X 1', city: 'Banfield', sports: ['padel'] }],
  });

  assert.deepEqual(parsed, {
    hub: { slug: 'banfield', name: 'Banfield' },
    complexes: [{ slug: 'club-sur', name: 'Club Sur', address: 'Av. X 1', sports: ['padel'] }],
  });
});

test('a null address shows no line instead of failing the page', () => {
  const parsed = parseHubData({
    hub: { slug: 'banfield', name: 'Banfield' },
    complexes: [{ slug: 'club-sur', name: 'Club Sur', address: null, sports: null }],
  });

  assert.equal(parsed.complexes[0].address, '');
  assert.deepEqual(parsed.complexes[0].sports, []);
});

test('parseHubData rejects a payload without a hub slug', () => {
  assert.throws(() => parseHubData({ hub: { name: 'Banfield' }, complexes: [] }), /invalid hub payload/);
});

test('parseHubData rejects a body that is not an object', () => {
  assert.throws(() => parseHubData(['not', 'a', 'hub']), /invalid hub payload/);
  assert.throws(() => parseHubData(null), /invalid hub payload/);
});
