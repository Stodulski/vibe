/**
 * Markdown twins of the server-rendered storefront pages: the city index, a city hub and a
 * complex. Each builder reads the same view the HTML page renders, so the two cannot say
 * different things. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseHubData, toHubView } from '../src/lib/hub.ts';
import { parseHubIndex, toHubIndexView } from '../src/lib/hub-index.ts';
import { hubMarkdown } from '../src/lib/markdown/hub.ts';
import { hubIndexMarkdown } from '../src/lib/markdown/hub-index.ts';
import { complexMarkdown, goneMarkdown } from '../src/lib/markdown/complex.ts';
import { goneView } from '../src/lib/complex-gone.ts';
import { markdownResponse } from '../src/lib/markdown/response.ts';
import { robotsDirective } from '../src/lib/storefront.ts';
import { complexFactsOf, type ComplexPayload, type ComplexLabels } from '../src/lib/markdown/complex-facts.ts';
import { escapeInline, frontMatter } from '../src/lib/markdown/format.ts';
import { complexHead } from '../src/lib/complex-meta.ts';

const SITE = 'https://vibe.com.ar';

const hubRaw = {
  hub: {
    slug: 'rosario',
    name: 'Rosario',
    province: 'Santa Fe',
    court_count: 5,
    sports: [
      { sport: 'padel', court_count: 3 },
      { sport: 'tennis', court_count: 2 },
    ],
  },
  complexes: [
    {
      slug: 'la-zona-f5',
      name: 'La Zona F5',
      address: 'Av. Pellegrini 1234',
      cover_url: null,
      sports: ['Pádel', 'Tenis'],
      court_count: 5,
      courts_by_sport: [
        { sport: 'padel', court_count: 3 },
        { sport: 'tennis', court_count: 2 },
      ],
    },
    {
      slug: 'club-norte',
      name: 'Club [Norte] *Pro*',
      address: '',
      cover_url: null,
      sports: ['Pádel'],
      court_count: 2,
      courts_by_sport: [{ sport: 'padel', court_count: 2 }],
    },
  ],
  other_hubs: [{ slug: 'cordoba', name: 'Córdoba', province: 'Córdoba', complex_count: 2 }],
};

test('a city hub is a Markdown document with one H1 and H2 sections, in order', () => {
  const view = toHubView(parseHubData(hubRaw));
  const md = hubMarkdown(view);
  const headings = md.split('\n').filter((line) => /^#{1,6} /.test(line));

  assert.equal(headings[0], `# ${view.h1}`);
  // A heading never skips a level: an H3 must sit under an H2 (or H1), not directly under the H1.
  const levels = headings.map((line) => line.match(/^#+/)?.[0].length ?? 0);
  levels.forEach((level, i) => {
    if (i > 0) assert.ok(level <= levels[i - 1] + 1, `heading ${headings[i]} skips a level`);
  });
  assert.ok(headings.includes('## Complejos en Rosario'));
  assert.ok(headings.includes('## Preguntas frecuentes'));
});

test('a city hub carries the canonical URL, the title and the description in its front matter', () => {
  const view = toHubView(parseHubData(hubRaw));
  const md = hubMarkdown(view);
  assert.ok(md.startsWith('---\n'));
  assert.ok(md.includes(`canonical: ${SITE}/canchas/rosario`));
  assert.ok(md.includes(`title: ${JSON.stringify(view.title)}`));
  assert.ok(md.includes(`description: ${JSON.stringify(view.description)}`));
});

test('every link in a city hub is absolute and points at the storefront', () => {
  const md = hubMarkdown(toHubView(parseHubData(hubRaw)));
  assert.ok(md.includes(`[La Zona F5](${SITE}/c/la-zona-f5)`));
  assert.ok(md.includes(`[Canchas en Córdoba](${SITE}/canchas/cordoba)`));
  assert.equal(/\]\(\//.test(md), false, 'no relative link survives');
  assert.equal(/\]\(#/.test(md), false, 'no fragment link survives');
});

test('a complex line states the address and the court counts per sport', () => {
  const md = hubMarkdown(toHubView(parseHubData(hubRaw)));
  assert.match(md, /La Zona F5\]\(https:\/\/vibe\.com\.ar\/c\/la-zona-f5\)[^\n]*Av\. Pellegrini 1234/);
  assert.match(md, /3 canchas de pádel · 2 de tenis/);
});

test('names with markdown syntax are escaped so the link text stays a link', () => {
  const md = hubMarkdown(toHubView(parseHubData(hubRaw)));
  assert.ok(md.includes('[Club \\[Norte\\] \\*Pro\\*](https://vibe.com.ar/c/club-norte)'));
});

test('a city hub writes its FAQ as Q and A under the FAQ heading', () => {
  const view = toHubView(parseHubData(hubRaw));
  const md = hubMarkdown(view);
  for (const item of view.faq) {
    assert.ok(md.includes(`### ${escapeInline(item.question)}`), item.question);
  }
});

test('a city hub with no complexes asks its owners to sign up instead of printing an empty list', () => {
  const view = toHubView(parseHubData({ ...hubRaw, complexes: [] }));
  const md = hubMarkdown(view);
  const headings = md.split('\n').filter((line) => /^#{1,6} /.test(line));

  assert.equal(headings[0], '# Canchas en Rosario');
  assert.equal(md.includes('## Complejos en Rosario'), false, 'no complexes section');
  assert.equal(md.includes('## Preguntas frecuentes'), false, 'no FAQ');
  assert.ok(md.includes('Todavía no hay canchas para reservar en Rosario con Vibe.'));
  assert.ok(md.includes('## ¿Tenés un complejo en Rosario?'));
  assert.ok(md.includes('[Sumar mi complejo gratis](https://app.vibe.com.ar/register)'));
  assert.ok(md.includes('## Otras ciudades con canchas'));
  assert.ok(md.includes(`- [Canchas en Córdoba](${SITE}/canchas/cordoba): 2 complejos`));
  assert.ok(md.includes(`[Todas las ciudades](${SITE}/canchas)`));
  assert.equal(/!/.test(md), false, 'no exclamation mark');
  assert.equal(/\]\(\//.test(md), false, 'no relative link survives');
});

test('the city index lists every city by province as absolute links', () => {
  const index = toHubIndexView(
    parseHubIndex({
      hubs: [
        { slug: 'rosario', name: 'Rosario', province: 'Santa Fe', complex_count: 2, sports: ['padel', 'tennis'] },
        { slug: 'cordoba', name: 'Córdoba', province: 'Córdoba', complex_count: 1, sports: ['padel'] },
      ],
    }),
  );
  const md = hubIndexMarkdown(index);
  assert.ok(md.startsWith('---\n'));
  assert.ok(md.includes(`# ${index.h1}`));
  assert.ok(md.includes(`## ${index.groups[0]?.heading}`));
  assert.ok(md.includes(`- [Canchas en Rosario](${SITE}/canchas/rosario)`));
  assert.ok(md.includes('2 complejos'));
  assert.equal(/\]\(\//.test(md), false);
});

const complexPayload: ComplexPayload = {
  complex: {
    name: 'La Zona F5',
    slug: 'la-zona-f5',
    address: 'Av. Pellegrini 1234',
    city: 'Rosario',
    city_slug: 'rosario',
    province: 'Santa Fe',
    phone: '+54 9 11 2463-8281',
    description: 'Canchas de pádel techadas con iluminación LED.',
    amenities: ['parking', 'bar'],
    payments_enabled: true,
    cover_url: null,
    logo_url: null,
  },
  courts: [
    {
      name: 'Cancha 1',
      sport: 'padel',
      is_active: true,
      prices: [
        { price: 1500000, day_type: 'monday', time_from: '08:00:00', time_to: '12:00:00' },
        { price: 1500000, day_type: 'tuesday', time_from: '08:00:00', time_to: '12:00:00' },
      ],
    },
    {
      name: 'Cancha 2',
      sport: 'padel',
      is_active: true,
      prices: [{ price: 1500000, day_type: 'monday', time_from: '08:00:00', time_to: '12:00:00' }],
    },
    { name: 'Cancha 3', sport: 'tennis', is_active: false, prices: [] },
  ],
  schedules: [
    { day: 'monday', open_time: '08:00:00', close_time: '23:00:00', is_closed: false },
    { day: 'sunday', open_time: '00:00:00', close_time: '00:00:00', is_closed: true },
  ],
};

const labels: ComplexLabels = {
  sport: (key) => ({ padel: 'Pádel', tennis: 'Tenis' })[key] ?? key,
  weekday: (key) => ({ monday: 'lunes', tuesday: 'martes', sunday: 'domingo' })[key] ?? key,
  amenity: (key) => ({ parking: 'Estacionamiento', bar: 'Bar' })[key] ?? key,
  price: (cents) => `$ ${cents / 100}`,
};

test('a complex page states its name as the H1 and its booking URL as a link', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('\n# La Zona F5\n'));
  assert.ok(md.includes(`${SITE}/c/la-zona-f5`));
});

test('a complex page links to no city hub and no city index, though its city is known', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.equal(complexPayload.complex.city_slug, 'rosario');
  assert.equal(md.includes('/canchas'), false, 'no link to /canchas/<city> or /canchas');
  assert.equal(md.includes('Todas las ciudades'), false);
  assert.equal(md.includes('Canchas en Rosario'), false);
});

test('a complex page lists active courts by sport and leaves out inactive ones', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('- Pádel: 2 canchas'));
  assert.equal(md.includes('Tenis'), false, 'the inactive tennis court is not listed');
});

test('a complex page lists its hours per day, and a closed day as closed', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('- lunes: 08:00 a 23:00'));
  assert.ok(md.includes('- domingo: cerrado'));
});

test('a complex page groups equal price bands across days and formats the amount', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('- Pádel, lunes y martes, 08:00 a 12:00: $ 15000'));
  // The same band on two courts is one line, not two.
  assert.equal(md.split('08:00 a 12:00').length - 1, 1);
});

test('a complex page gives its WhatsApp link from the phone digits, as the booking page does', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('https://wa.me/5491124638281'));
});

test('a complex that takes payments online says so, and one that does not says WhatsApp', () => {
  const online = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(online.includes('Reservá online'));
  const offline = complexMarkdown(
    complexFactsOf('la-zona-f5', { ...complexPayload, complex: { ...complexPayload.complex, payments_enabled: false } }, labels),
  );
  assert.ok(offline.includes('Reservá por WhatsApp'));
  assert.equal(offline.includes('Reservá online'), false);
});

test('a complex page answers with the amenity labels and a data-only FAQ', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(md.includes('Estacionamiento'));
  assert.ok(md.includes('### ¿Dónde queda La Zona F5?'));
  assert.ok(md.includes('### ¿Cómo reservo en La Zona F5?'));
});

test('the complex description comes from the complex, or from the page copy when it has none', () => {
  const withText = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, labels));
  assert.ok(withText.includes('Canchas de pádel techadas con iluminación LED.'));
  const bare = complexMarkdown(
    complexFactsOf('la-zona-f5', { ...complexPayload, complex: { ...complexPayload.complex, description: '' } }, labels),
  );
  const pageCopy = complexHead({
    slug: 'la-zona-f5',
    name: 'La Zona F5',
    city: 'Rosario',
    online: true,
    sports: ['Pádel'],
    services: ['Estacionamiento', 'Bar'],
    courtCount: 2,
    ownImage: null,
  }).description;
  assert.ok(bare.includes(pageCopy), 'the page meta description is the fallback');
});

test('the frontmatter quotes values that carry colons or quotes', () => {
  const fm = frontMatter({ title: 'Canchas: "Rosario"', canonical: `${SITE}/canchas/rosario` });
  assert.equal(fm.split('\n')[1], 'title: "Canchas: \\"Rosario\\""');
});

test('markdown never carries raw HTML from a name', () => {
  const md = complexMarkdown(
    complexFactsOf('la-zona-f5', { ...complexPayload, complex: { ...complexPayload.complex, name: 'Club <b>Sur</b>' } }, labels),
  );
  // The body is what a renderer shows; an unescaped "<" there opens a tag. The front matter
  // is YAML data, not rendered Markdown, so it is checked by the frontmatter tests instead.
  const body = md.slice(md.indexOf('\n---\n') + 5);
  assert.equal(/(^|[^\\])<[a-zA-Z/]/m.test(body), false);
});

test('a price band open every day says so, instead of naming seven days', () => {
  const everyDay = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];
  const payload = {
    ...complexPayload,
    courts: [
      {
        name: 'Cancha 1',
        sport: 'padel',
        is_active: true,
        prices: everyDay.map((day) => ({ price: 1500000, day_type: day, time_from: '08:00:00', time_to: '12:00:00' })),
      },
    ],
  };
  const md = complexMarkdown(complexFactsOf('la-zona-f5', payload, labels));
  assert.ok(md.includes('- Pádel, todos los días, 08:00 a 12:00: $ 15000'));
});

test('weekday names inside a price line are lowercase, as they read in a sentence', () => {
  const md = complexMarkdown(complexFactsOf('la-zona-f5', complexPayload, { ...labels, weekday: (key) => ({ monday: 'Lunes', tuesday: 'Martes' })[key] ?? key }));
  assert.ok(md.includes('- Pádel, lunes y martes, 08:00 a 12:00: $ 15000'));
});

test('when one band carries two prices, each line names its court so the two are not confused', () => {
  const payload = {
    ...complexPayload,
    courts: [
      { name: 'Cancha 1', sport: 'padel', is_active: true, prices: [{ price: 1500000, day_type: 'monday', time_from: '08:00:00', time_to: '12:00:00' }] },
      { name: 'Cancha 3', sport: 'padel', is_active: true, prices: [{ price: 1800000, day_type: 'monday', time_from: '08:00:00', time_to: '12:00:00' }] },
    ],
  };
  const md = complexMarkdown(complexFactsOf('la-zona-f5', payload, labels));
  assert.ok(md.includes('- Pádel, lunes, 08:00 a 12:00 (Cancha 1): $ 15000'));
  assert.ok(md.includes('- Pádel, lunes, 08:00 a 12:00 (Cancha 3): $ 18000'));
});

const deleted = { name: 'Club Viejo', city: 'Rosario', citySlug: 'rosario' };
const deletedUrl = `${SITE}/c/club-viejo`;

test('the Markdown twin of a deleted complex names it as gone and lists the other courts of its city', () => {
  const md = goneMarkdown(goneView(deleted, parseHubData(hubRaw)), deletedUrl);
  assert.ok(md.includes('title: "Club Viejo ya no toma reservas por Vibe"'));
  assert.ok(md.includes('\n# Club Viejo ya no toma reservas por Vibe\n'));
  assert.ok(md.includes('Pero hay otras canchas en Rosario donde podés reservar online.'));
  assert.ok(md.includes('\n## Otras canchas en Rosario\n'));
  assert.ok(md.includes(`[La Zona F5](${SITE}/c/la-zona-f5)`));
  assert.ok(md.includes(`[Ver canchas en Rosario](${SITE}/canchas/rosario)`));
  assert.ok(md.includes(`[Ver todas las ciudades](${SITE}/canchas)`));
});

test('the Markdown twin of a deleted complex whose city has no courts says so, and keeps the city index', () => {
  const md = goneMarkdown(goneView(deleted, null), deletedUrl);
  assert.ok(md.includes('Mirá las canchas de otras ciudades.'));
  assert.equal(md.includes('## Otras canchas'), false);
  assert.equal(md.includes('/canchas/rosario'), false);
  assert.ok(md.includes(`[Ver todas las ciudades](${SITE}/canchas)`));
});

test('the 410 answer of the twin keeps the cache policy and the same robots rule as the HTML page', () => {
  const res = markdownResponse({ status: 410, body: '', canonical: deletedUrl, cacheControl: 'public, s-maxage=300' });
  assert.equal(res.status, 410);
  assert.equal(res.headers.get('Cache-Control'), 'public, s-maxage=300');
  assert.equal(res.headers.get('X-Robots-Tag'), robotsDirective(410));
});
