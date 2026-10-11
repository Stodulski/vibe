/**
 * Unit tests for the city hub view model: the payload contract, the title, H1,
 * description, intro, FAQ, breadcrumb and JSON-LD. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  breadcrumbJsonLd,
  citySlugOf,
  HUB_DESCRIPTION_MAX,
  hubDescription,
  hubFaq,
  hubH1,
  hubIntro,
  hubRobots,
  hubTitle,
  jsonLdScript,
  MIN_INDEXABLE_HUB_COMPLEXES,
  parseHubData,
  sportLabel,
  toHubView,
  type HubView,
} from '../src/lib/hub.ts';

const banfield = parseHubData({
  hub: {
    slug: 'banfield',
    name: 'Banfield',
    province: 'Buenos Aires',
    complex_count: 1,
    court_count: 3,
    sports: [
      { sport: 'Pádel', court_count: 1 },
      { sport: 'Tenis', court_count: 2 },
    ],
  },
  complexes: [
    {
      slug: 'club-sur',
      name: 'Club Sur',
      address: 'Av. Siempre Viva 742',
      city: 'Banfield',
      province: 'Buenos Aires',
      cover_url: 'https://img.example.com/club-sur.jpg',
      court_count: 3,
      sports: ['Tenis', 'Pádel'],
      courts_by_sport: [
        { sport: 'Tenis', court_count: 2 },
        { sport: 'Pádel', court_count: 1 },
      ],
    },
  ],
  other_hubs: [{ slug: 'rosario', name: 'Rosario', province: 'Santa Fe', complex_count: 2 }],
});

const jsonLdOf = (view: HubView) => view.jsonLd.map((block) => JSON.parse(block));

// ---- title and H1 ----------------------------------------------------------

test('the title names the sports, the city and the province', () => {
  assert.equal(hubTitle('Banfield', 'Buenos Aires', ['Pádel']), 'Canchas de pádel en Banfield, Buenos Aires | Vibe');
});

test('the title drops the province when it would exceed 60 characters', () => {
  assert.equal(
    hubTitle('Caleta Olivia', 'Santa Cruz', ['Fútbol', 'Pádel']),
    'Canchas de fútbol y pádel en Caleta Olivia | Vibe',
  );
});

test('the title keeps only the top sport when the city name alone is too long', () => {
  assert.equal(
    hubTitle('Villa Gobernador Gálvez con nombre largo de ciudad', 'Santa Fe', ['Fútbol', 'Pádel', 'Tenis']),
    'Canchas de fútbol en Villa Gobernador Gálvez con nombre largo de ciudad | Vibe',
  );
});

test('the title names at most three sports', () => {
  assert.equal(
    hubTitle('Rosario', '', ['Fútbol', 'Pádel', 'Tenis', 'Vóley']),
    'Canchas de fútbol, pádel y tenis en Rosario | Vibe',
  );
});

test('the title drops the province when none is known', () => {
  assert.equal(hubTitle('Rosario', '', ['Pádel']), 'Canchas de pádel en Rosario | Vibe');
});

test('the title without sports is the plain city title', () => {
  assert.equal(hubTitle('Rosario', 'Santa Fe', []), 'Canchas en Rosario | Vibe');
});

test('the H1 names the sports and the city, never the province', () => {
  assert.equal(hubH1('Caleta Olivia', ['Fútbol', 'Pádel']), 'Canchas de fútbol y pádel en Caleta Olivia');
  assert.equal(hubH1('Rosario', []), 'Canchas en Rosario');
});

// ---- meta description ------------------------------------------------------

test('the description names the sports, city, province and the complex', () => {
  assert.equal(
    hubDescription('Banfield', 'Buenos Aires', ['Pádel'], ['Club Sur']),
    'Reservá canchas de pádel en Banfield, Buenos Aires. 1 complejo: Club Sur. Mirá los turnos libres y reservá online.',
  );
});

test('two complexes are joined with "y"', () => {
  assert.equal(
    hubDescription('Banfield', '', ['Pádel'], ['A', 'B']),
    'Reservá canchas de pádel en Banfield. 2 complejos: A y B. Mirá los turnos libres y reservá online.',
  );
});

test('more than three complexes name three and count the rest', () => {
  assert.equal(
    hubDescription('Banfield', '', [], ['A', 'B', 'C', 'D', 'E']),
    'Reservá canchas en Banfield. 5 complejos: A, B, C y 2 más. Mirá los turnos libres y reservá online.',
  );
});

test('a long description is cut at a word boundary within the limit', () => {
  assert.equal(HUB_DESCRIPTION_MAX, 155);
  const names = Array.from({ length: 3 }, (_, i) => `Complejo ${'x'.repeat(60)} ${i}`);
  const description = hubDescription('Banfield', 'Buenos Aires', ['Pádel', 'Tenis'], names);
  const full = `Reservá canchas de pádel y tenis en Banfield, Buenos Aires. 3 complejos: ${names[0]}, ${names[1]} y ${names[2]}. Mirá los turnos libres y reservá online.`;

  assert.ok(Array.from(description).length <= HUB_DESCRIPTION_MAX);
  assert.ok(description.endsWith('...'));
  assert.ok(full.startsWith(description.slice(0, -3)));
  assert.equal(full.charAt(description.length - 3), ' ');
});

test('a cut never splits a multi-byte letter', () => {
  const description = hubDescription('Banfield', '', [], [`Ñandú ${'ñ'.repeat(200)}`]);

  assert.ok(Array.from(description).length <= HUB_DESCRIPTION_MAX);
  assert.ok(!description.includes('�'));
});

// ---- intro -----------------------------------------------------------------

test('the intro states the complexes, the courts and the sports from the data', () => {
  assert.equal(
    hubIntro('Caleta Olivia', 2, 4, ['Fútbol', 'Pádel']),
    'En Caleta Olivia hay 2 complejos con 4 canchas de fútbol y pádel que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
});

test('the intro leaves out the court count when it is unknown', () => {
  assert.equal(
    hubIntro('Caleta Olivia', 1, null, ['Pádel']),
    'En Caleta Olivia hay 1 complejo con canchas de pádel que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
});

test('the intro with no sports and no courts names only the complexes', () => {
  assert.equal(
    hubIntro('Rosario', 1, null, []),
    'En Rosario hay 1 complejo que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
});

test('a single court is singular', () => {
  assert.equal(
    hubIntro('Rosario', 1, 1, ['Pádel']),
    'En Rosario hay 1 complejo con 1 cancha de pádel que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
});

// ---- FAQ -------------------------------------------------------------------

test('the FAQ asks where each sport is played, from the complexes that offer it', () => {
  assert.deepEqual(
    hubFaq('Caleta Olivia', ['Fútbol', 'Pádel'], [
      { name: 'La Zona F5', sports: ['Fútbol', 'Pádel'] },
      { name: 'Club Sur', sports: ['Pádel'] },
    ]),
    [
      {
        question: '¿Dónde hay canchas de fútbol en Caleta Olivia?',
        answer: 'En Caleta Olivia podés reservar canchas de fútbol en La Zona F5.',
      },
      {
        question: '¿Dónde hay canchas de pádel en Caleta Olivia?',
        answer: 'En Caleta Olivia podés reservar canchas de pádel en La Zona F5 y Club Sur.',
      },
      {
        question: '¿Cómo reservo una cancha en Caleta Olivia?',
        answer: 'Elegí un complejo de la lista, mirá los turnos libres y confirmá tu reserva online.',
      },
      {
        question: '¿Cuántos complejos de Caleta Olivia reciben reservas en Vibe?',
        answer: '2 complejos: La Zona F5 y Club Sur.',
      },
    ],
  );
});

test('the FAQ asks about at most three sports', () => {
  const questions = hubFaq('Rosario', ['Fútbol', 'Pádel', 'Tenis', 'Vóley'], [
    { name: 'A', sports: ['Fútbol', 'Pádel', 'Tenis', 'Vóley'] },
  ]).map((item) => item.question);

  assert.deepEqual(questions.filter((q) => q.startsWith('¿Dónde hay')), [
    '¿Dónde hay canchas de fútbol en Rosario?',
    '¿Dónde hay canchas de pádel en Rosario?',
    '¿Dónde hay canchas de tenis en Rosario?',
  ]);
});

test('a sport that no complex offers gets no question', () => {
  const faq = hubFaq('Rosario', ['Tenis'], [{ name: 'A', sports: ['Pádel'] }]);

  assert.equal(faq.length, 2);
  assert.ok(faq.every((item) => !item.question.startsWith('¿Dónde hay')));
});

test('one complex is counted in the singular', () => {
  const faq = hubFaq('Rosario', [], [{ name: 'A', sports: [] }]);

  assert.deepEqual(faq.at(-1), {
    question: '¿Cuántos complejos de Rosario reciben reservas en Vibe?',
    answer: '1 complejo: A.',
  });
});

// ---- breadcrumb and JSON-LD ------------------------------------------------

test('the breadcrumb is a BreadcrumbList with positions and absolute URLs', () => {
  const list = JSON.parse(
    breadcrumbJsonLd([
      { name: 'Inicio', url: 'https://vibe.com.ar/' },
      { name: 'Canchas', url: 'https://vibe.com.ar/canchas' },
      { name: 'Banfield', url: 'https://vibe.com.ar/canchas/banfield' },
    ]),
  );

  assert.equal(list['@context'], 'https://schema.org');
  assert.equal(list['@type'], 'BreadcrumbList');
  assert.deepEqual(list.itemListElement, [
    { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar/' },
    { '@type': 'ListItem', position: 2, name: 'Canchas', item: 'https://vibe.com.ar/canchas' },
    { '@type': 'ListItem', position: 3, name: 'Banfield', item: 'https://vibe.com.ar/canchas/banfield' },
  ]);
});

test('the view carries the title, H1, intro, description and canonical from the data', () => {
  const view = toHubView(banfield);

  assert.equal(view.title, 'Canchas de tenis y pádel en Banfield, Buenos Aires | Vibe');
  assert.equal(view.h1, 'Canchas de tenis y pádel en Banfield');
  assert.equal(
    view.description,
    'Reservá canchas de tenis y pádel en Banfield, Buenos Aires. 1 complejo: Club Sur. Mirá los turnos libres y reservá online.',
  );
  assert.equal(
    view.intro,
    'En Banfield hay 1 complejo con 3 canchas de tenis y pádel que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
  assert.equal(view.canonical, 'https://vibe.com.ar/canchas/banfield');
  assert.equal(view.ogImage, 'https://img.example.com/club-sur.jpg');
});

test('a complex card carries its cover, labelled sports and the courts per sport', () => {
  const [complex] = toHubView(banfield).complexes;

  assert.equal(complex.href, '/c/club-sur');
  assert.equal(complex.address, 'Av. Siempre Viva 742');
  assert.equal(complex.coverUrl, 'https://img.example.com/club-sur.jpg');
  assert.deepEqual(complex.sports, ['Tenis', 'Pádel']);
  assert.equal(complex.courtsLine, '2 canchas de tenis · 1 de pádel');
});

test('the view links the other cities', () => {
  assert.deepEqual(toHubView(banfield).otherHubs, [
    { name: 'Rosario', href: '/canchas/rosario', label: 'Canchas en Rosario' },
  ]);
});

test('the FAQ in the view is the one the FAQPage JSON-LD repeats, word for word', () => {
  const view = toHubView(banfield);
  const [, , faqPage] = jsonLdOf(view);

  assert.equal(faqPage['@type'], 'FAQPage');
  assert.deepEqual(
    faqPage.mainEntity.map((entry: { name: string; acceptedAnswer: { text: string } }) => ({
      question: entry.name,
      answer: entry.acceptedAnswer.text,
    })),
    view.faq,
  );
});

test('the collection JSON-LD describes the page, its place and the complexes as an ItemList', () => {
  const [collection] = jsonLdOf(toHubView(banfield));

  assert.equal(collection['@context'], 'https://schema.org');
  assert.equal(collection['@type'], 'CollectionPage');
  assert.equal(collection.name, 'Canchas de tenis y pádel en Banfield');
  assert.equal(collection.url, 'https://vibe.com.ar/canchas/banfield');
  assert.deepEqual(collection.about, {
    '@type': 'Place',
    name: 'Banfield',
    address: {
      '@type': 'PostalAddress',
      addressLocality: 'Banfield',
      addressRegion: 'Buenos Aires',
      addressCountry: 'AR',
    },
  });
  assert.deepEqual(collection.mainEntity, {
    '@type': 'ItemList',
    name: 'Canchas de tenis y pádel en Banfield',
    itemListElement: [
      { '@type': 'ListItem', position: 1, url: 'https://vibe.com.ar/c/club-sur', name: 'Club Sur' },
    ],
  });
});

test('the breadcrumb of the hub runs Inicio, Canchas and the city', () => {
  const [, breadcrumb] = jsonLdOf(toHubView(banfield));

  assert.equal(breadcrumb['@type'], 'BreadcrumbList');
  assert.deepEqual(
    breadcrumb.itemListElement.map((item: { position: number; name: string; item: string }) => [
      item.position,
      item.name,
      item.item,
    ]),
    [
      [1, 'Inicio', 'https://vibe.com.ar/'],
      [2, 'Canchas', 'https://vibe.com.ar/canchas'],
      [3, 'Banfield', 'https://vibe.com.ar/canchas/banfield'],
    ],
  );
});

test('JSON-LD escapes characters that could close the script element', () => {
  const hostile = toHubView(parseHubData({ hub: { slug: 'banfield', name: '</script><b>&' }, complexes: [] }));

  for (const block of hostile.jsonLd) assert.ok(!/[<>&]/.test(block));
  assert.equal(jsonLdOf(hostile)[0].mainEntity.name, 'Canchas en </script><b>&');
});

test('jsonLdScript round-trips line separators as valid JSON', () => {
  const value = { text: 'a b c' };
  assert.deepEqual(JSON.parse(jsonLdScript(value)), value);
});

// ---- payload contract ------------------------------------------------------

test('known sports get their Spanish label and unknown ones are shown as stored', () => {
  assert.equal(sportLabel('padel'), 'Pádel');
  assert.equal(sportLabel('basketball'), 'Básquet');
  assert.equal(sportLabel('Fútbol'), 'Fútbol');
  assert.equal(sportLabel('squash'), 'squash');
  assert.equal(sportLabel('constructor'), 'constructor');
});

test('parseHubData reads the new fields and keeps the labels of the sports', () => {
  assert.equal(banfield.hub.province, 'Buenos Aires');
  assert.equal(banfield.hub.courtCount, 3);
  assert.deepEqual(banfield.hub.sports, [
    { sport: 'Pádel', courtCount: 1 },
    { sport: 'Tenis', courtCount: 2 },
  ]);
  assert.equal(banfield.complexes[0].courtCount, 3);
});

test('an older payload with no new fields still renders with the same rules', () => {
  const old = parseHubData({
    hub: { slug: 'banfield', name: 'Banfield' },
    complexes: [{ slug: 'club-sur', name: 'Club Sur', address: 'Av. X 1', city: 'Banfield', sports: ['Pádel'] }],
  });
  const view = toHubView(old);

  assert.equal(view.title, 'Canchas de pádel en Banfield | Vibe');
  assert.equal(view.h1, 'Canchas de pádel en Banfield');
  assert.equal(
    view.description,
    'Reservá canchas de pádel en Banfield. 1 complejo: Club Sur. Mirá los turnos libres y reservá online.',
  );
  assert.equal(
    view.intro,
    'En Banfield hay 1 complejo con canchas de pádel que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.',
  );
  assert.equal(view.ogImage, null);
  assert.equal(view.complexes[0].courtsLine, null);
  assert.deepEqual(view.complexes[0].sports, ['Pádel']);
  assert.deepEqual(view.otherHubs, []);
});

test('without hub.sports the sports are the union of the complexes', () => {
  const view = toHubView(
    parseHubData({
      hub: { slug: 'x', name: 'X' },
      complexes: [
        { slug: 'a', name: 'A', sports: ['Fútbol'] },
        { slug: 'b', name: 'B', sports: ['Fútbol', 'Pádel'] },
      ],
    }),
  );

  assert.equal(view.title, 'Canchas de fútbol y pádel en X | Vibe');
});

test('an empty hub.sports list means no sports, even when a complex lists some', () => {
  const view = toHubView(
    parseHubData({ hub: { slug: 'x', name: 'X', sports: [] }, complexes: [{ slug: 'a', name: 'A', sports: ['Pádel'] }] }),
  );

  assert.equal(view.title, 'Canchas en X | Vibe');
  assert.equal(view.h1, 'Canchas en X');
});

test('a cover that is not an http(s) URL is never used as an image', () => {
  const view = toHubView(
    parseHubData({
      hub: { slug: 'x', name: 'X' },
      complexes: [{ slug: 'a', name: 'A', cover_url: 'javascript:alert(1)' }],
    }),
  );

  assert.equal(view.ogImage, null);
  assert.equal(view.complexes[0].coverUrl, null);
});

test('other cities without a slug or a name are skipped, not fatal', () => {
  const data = parseHubData({
    hub: { slug: 'x', name: 'X' },
    complexes: [],
    other_hubs: [{ slug: 'rosario', name: 'Rosario' }, { name: 'Sin slug' }, 'x'],
  });

  assert.deepEqual(data.otherHubs.map((hub) => hub.slug), ['rosario']);
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

test('citySlugOf reads the city slug of a complex payload, and only a plain slug', () => {
  assert.equal(citySlugOf({ complex: { city_slug: 'caleta-olivia' } }), 'caleta-olivia');
  assert.equal(citySlugOf({ complex: {} }), null);
  assert.equal(citySlugOf({ complex: { city_slug: 42 } }), null);
  assert.equal(citySlugOf({ complex: { city_slug: '' } }), null);
  assert.equal(citySlugOf({ complex: { city_slug: 'Caleta Olivia' } }), null);
  assert.equal(citySlugOf({ complex: { city_slug: 'a/b' } }), null);
  assert.equal(citySlugOf(null), null);
});

test('a hub with fewer than two complexes is followed but not indexed', () => {
  assert.equal(MIN_INDEXABLE_HUB_COMPLEXES, 2);
  assert.equal(hubRobots(1), 'noindex, follow');
  assert.equal(hubRobots(0), 'noindex, follow');
  assert.equal(hubRobots(2), 'index, follow');
});

test('an error answer stays noindex whatever the number of complexes', () => {
  assert.equal(hubRobots(3, 404), 'noindex');
  assert.equal(hubRobots(1, 503), 'noindex');
});

test('the view carries the robots directive of its hub', () => {
  assert.equal(toHubView(banfield).robots, 'noindex, follow');
  const two = parseHubData({
    hub: { slug: 'x', name: 'X' },
    complexes: [
      { slug: 'a', name: 'A' },
      { slug: 'b', name: 'B' },
    ],
  });
  assert.equal(toHubView(two).robots, 'index, follow');
});

test('names from the payload are trimmed and their inner spaces collapsed', () => {
  const parsed = parseHubData({
    hub: { slug: 'rosario', name: 'Rosario ', province: ' Santa  Fe ' },
    complexes: [{ slug: 'afa', name: 'Afa ', address: 'Av.  X 1 ' }],
  });

  assert.equal(parsed.hub.name, 'Rosario');
  assert.equal(parsed.hub.province, 'Santa Fe');
  assert.equal(parsed.complexes[0].name, 'Afa');
  assert.equal(parsed.complexes[0].address, 'Av. X 1');
});

test('the copy never shows a stray space before a comma or a period', () => {
  assert.equal(hubDescription('Rosario', '', [], ['Afa ']), 'Reservá canchas en Rosario. 1 complejo: Afa. Mirá los turnos libres y reservá online.');
  assert.equal(hubTitle('Rosario ', ' Santa Fe ', ['Pádel']), 'Canchas de pádel en Rosario, Santa Fe | Vibe');
  assert.equal(hubH1(' Rosario ', []), 'Canchas en Rosario');
});
