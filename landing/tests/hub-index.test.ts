/**
 * Unit tests for the city index at /canchas: the payload contract, the grouping by
 * province and the JSON-LD. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseHubIndex, toHubIndexView } from '../src/lib/hub-index.ts';

const index = parseHubIndex({
  hubs: [
    { slug: 'caleta-olivia', name: 'Caleta Olivia', province: 'Santa Cruz', complex_count: 1, sports: ['Fútbol', 'Pádel'] },
    { slug: 'banfield', name: 'Banfield', province: 'Buenos Aires', complex_count: 2, sports: ['Pádel'] },
    { slug: 'rosario', name: 'Rosario', province: 'Santa Fe', complex_count: 1, sports: [] },
  ],
});

test('the index keeps every city of the payload, in its order', () => {
  assert.deepEqual(
    index.hubs.map((hub) => hub.slug),
    ['caleta-olivia', 'banfield', 'rosario'],
  );
});

test('a payload that is not a list of cities is refused', () => {
  assert.throws(() => parseHubIndex({ cities: [] }), /invalid hub payload/);
  assert.throws(() => parseHubIndex({ hubs: [{ name: 'Banfield' }] }), /invalid hub payload/);
  assert.throws(() => parseHubIndex(null), /invalid hub payload/);
});

test('a city name with stray spaces is trimmed before it reaches the links', () => {
  const [hub] = parseHubIndex({ hubs: [{ slug: 'afa', name: 'Afa ', province: ' Santa  Fe ', complex_count: 1 }] }).hubs;

  assert.equal(hub.name, 'Afa');
  assert.equal(hub.province, 'Santa Fe');
  assert.equal(toHubIndexView(parseHubIndex({ hubs: [{ slug: 'afa', name: 'Afa ', complex_count: 1 }] })).groups[0].hubs[0].label, 'Canchas en Afa');
});

test('a city without counts or sports still parses, with empty defaults', () => {
  const [hub] = parseHubIndex({ hubs: [{ slug: 'x', name: 'X' }] }).hubs;

  assert.deepEqual(hub, { slug: 'x', name: 'X', province: '', complexCount: 0, sports: [] });
});

test('the index has its fixed title, H1 and canonical, and an intro from the data', () => {
  const view = toHubIndexView(index);

  assert.equal(view.title, 'Canchas para reservar por ciudad | Vibe');
  assert.equal(view.h1, 'Canchas por ciudad');
  assert.equal(view.canonical, 'https://vibe.com.ar/canchas');
  assert.equal(view.intro, 'Elegí tu ciudad y reservá en 4 complejos de 3 ciudades.');
});

test('the description names the cities, in display order', () => {
  assert.equal(
    toHubIndexView(index).description,
    'Canchas en Banfield, Caleta Olivia y Rosario. Elegí tu ciudad y reservá online en Vibe.',
  );
});

test('the intro is singular for one complex in one city', () => {
  const view = toHubIndexView(parseHubIndex({ hubs: [{ slug: 'x', name: 'X', province: '', complex_count: 1 }] }));

  assert.equal(view.intro, 'Elegí tu ciudad y reservá en 1 complejo de 1 ciudad.');
});

test('cities are grouped by province in order, with cities without one under "Otras" last', () => {
  const view = toHubIndexView(
    parseHubIndex({
      hubs: [
        { slug: 'zapala', name: 'Zapala', province: 'Neuquén', complex_count: 1 },
        { slug: 'sin', name: 'Sin Provincia', province: '', complex_count: 1 },
        { slug: 'rosario', name: 'Rosario', province: 'Santa Fe', complex_count: 1 },
        { slug: 'banfield', name: 'Banfield', province: 'Buenos Aires', complex_count: 2, sports: ['Pádel'] },
        { slug: 'avellaneda', name: 'Avellaneda', province: 'Buenos Aires', complex_count: 1 },
      ],
    }),
  );

  assert.deepEqual(
    view.groups.map((group) => group.heading),
    ['Buenos Aires', 'Neuquén', 'Santa Fe', 'Otras'],
  );
  assert.deepEqual(
    view.groups[0].hubs.map((hub) => hub.name),
    ['Avellaneda', 'Banfield'],
  );
});

test('each city links its hub with its sports and its complex count', () => {
  const [buenosAires, santaCruz, santaFe] = toHubIndexView(index).groups;

  assert.deepEqual(buenosAires.hubs[0], {
    slug: 'banfield',
    name: 'Banfield',
    href: '/canchas/banfield',
    label: 'Canchas en Banfield',
    sportsLine: 'Pádel',
    complexLine: '2 complejos',
  });
  assert.equal(santaCruz.hubs[0].sportsLine, 'Fútbol y Pádel');
  assert.equal(santaCruz.hubs[0].complexLine, '1 complejo');
  assert.equal(santaFe.hubs[0].sportsLine, '');
});

test('the JSON-LD is an ItemList in display order, and a breadcrumb', () => {
  const [list, breadcrumb] = toHubIndexView(index).jsonLd.map((block) => JSON.parse(block));

  assert.equal(list['@type'], 'ItemList');
  assert.equal(list.name, 'Canchas por ciudad');
  assert.deepEqual(
    list.itemListElement.map((item: { position: number; name: string; url: string }) => [
      item.position,
      item.name,
      item.url,
    ]),
    [
      [1, 'Canchas en Banfield', 'https://vibe.com.ar/canchas/banfield'],
      [2, 'Canchas en Caleta Olivia', 'https://vibe.com.ar/canchas/caleta-olivia'],
      [3, 'Canchas en Rosario', 'https://vibe.com.ar/canchas/rosario'],
    ],
  );
  assert.equal(breadcrumb['@type'], 'BreadcrumbList');
  assert.deepEqual(
    breadcrumb.itemListElement.map((item: { name: string }) => item.name),
    ['Inicio', 'Canchas'],
  );
});

test('an empty index says so and keeps valid JSON-LD', () => {
  const view = toHubIndexView(parseHubIndex({ hubs: [] }));

  assert.equal(view.intro, 'Todavía no hay ciudades con complejos de Vibe.');
  assert.deepEqual(view.groups, []);
  assert.equal(JSON.parse(view.jsonLd[0]).itemListElement.length, 0);
});

test('a hostile city or sport name cannot close the script element', () => {
  const view = toHubIndexView(
    parseHubIndex({
      hubs: [{ slug: 'x', name: '</script>', province: '', complex_count: 1, sports: ['&'] }],
    }),
  );

  for (const block of view.jsonLd) assert.ok(!/[<>&]/.test(block));
});
