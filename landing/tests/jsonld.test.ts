/* JSON-LD de la landing: lo que Google mira en los articulos, la home sin
   HowTo retirado, y la raiz del sitio escrita igual que su canonical. Run with
   `pnpm test:unit`. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  homeJsonLd,
  privacidadJsonLd,
  terminosJsonLd,
  guiaJsonLd,
  guiasIndexJsonLd,
  tutorialJsonLd,
  tutorialesIndexJsonLd,
} from '../src/data/jsonld.ts';
import { readFileSync } from 'node:fs';
import { PUBLISHED } from '../src/data/site-dates.ts';
import { guias } from '../src/data/guias.ts';
import { tutoriales } from '../src/data/tutoriales.ts';

type Bloque = Record<string, unknown>;

const parsear = (bloques: string[]): Bloque[] => bloques.map(b => JSON.parse(b));
const articuloDe = (bloques: string[]) => parsear(bloques).find(b => b['@type'] === 'Article');

function valores(v: unknown): unknown[] {
  if (Array.isArray(v)) return v.flatMap(valores);
  if (v && typeof v === 'object') return Object.values(v).flatMap(valores);
  return [v];
}

test('every guia Article declares an absolute image, as Google requires for Article', () => {
  for (const g of guias) {
    const articulo = articuloDe(guiaJsonLd(g));
    assert.ok(articulo, `${g.slug} sin Article`);
    assert.match(String(articulo.image), /^https:\/\/vibe\.com\.ar\/.+\.(png|jpg|webp)$/, g.slug);
  }
});

test('every tutorial Article declares an absolute image, as Google requires for Article', () => {
  for (const t of tutoriales) {
    const articulo = articuloDe(tutorialJsonLd(t));
    assert.ok(articulo, `${t.slug} sin Article`);
    assert.match(String(articulo.image), /^https:\/\/vibe\.com\.ar\/.+\.(png|jpg|webp)$/, t.slug);
  }
});

test('the home no longer ships a HowTo block: Google retired its rich results', () => {
  assert.equal(parsear(homeJsonLd).some(b => b['@type'] === 'HowTo'), false);
});

test('the home keeps its FAQPage block', () => {
  assert.equal(parsear(homeJsonLd).some(b => b['@type'] === 'FAQPage'), true);
});

test('the site root is written once, with its trailing slash, like the home canonical', () => {
  const bloques = [
    ...homeJsonLd,
    privacidadJsonLd,
    terminosJsonLd,
    guiasIndexJsonLd(guias),
    tutorialesIndexJsonLd(tutoriales),
    guiaJsonLd(guias[0]),
    tutorialJsonLd(tutoriales[0]),
  ].flat();
  const raices = bloques.flatMap(b => valores(JSON.parse(b))).filter(v => v === 'https://vibe.com.ar');
  assert.deepEqual(raices, []);
});

/* ---------------------------------------------------- entity graph and dates */

const PAGE_DATES = JSON.parse(readFileSync(new URL('../src/data/page-dates.json', import.meta.url), 'utf8')) as Record<
  string,
  { date: string }
>;
const ORGANIZATION = 'https://vibe.com.ar/#organization';
const WEBSITE = 'https://vibe.com.ar/#website';
const SOFTWARE = 'https://vibe.com.ar/#software';

/** Every object node in a JSON-LD value, at any depth. */
function nodos(v: unknown): Bloque[] {
  if (Array.isArray(v)) return v.flatMap(nodos);
  if (v && typeof v === 'object') return [v as Bloque, ...Object.values(v).flatMap(nodos)];
  return [];
}

/** A reference is a node that only points at another node by @id. */
const esReferencia = (v: unknown): v is { '@id': string } =>
  !!v && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 1 && typeof (v as Bloque)['@id'] === 'string';

test('the organization, the website and the software each define a stable @id on the home page', () => {
  const definidos = new Set(parsear(homeJsonLd).map(b => b['@id']).filter(Boolean));
  for (const id of [ORGANIZATION, WEBSITE, SOFTWARE]) assert.ok(definidos.has(id), `missing ${id}`);
});

test('every @id reference in the site resolves to an entity the home page defines', () => {
  const definidos = new Set(parsear(homeJsonLd).map(b => b['@id']).filter(Boolean));
  const todos = [
    ...homeJsonLd,
    ...privacidadJsonLd,
    ...terminosJsonLd,
    ...guiaJsonLd(guias[0]!),
    ...tutorialJsonLd(tutoriales[0]!),
    ...guiasIndexJsonLd(guias),
    ...tutorialesIndexJsonLd(tutoriales),
  ];
  const referencias = parsear(todos).flatMap(nodos).filter(esReferencia).map(r => r['@id']);
  assert.ok(referencias.length > 0, 'the site should reference the organization somewhere');
  for (const id of referencias) assert.ok(definidos.has(id), `dangling reference ${id}`);
});

test('the software, the website and every article point at the organization by @id, not by a copy', () => {
  const home = parsear(homeJsonLd);
  const software = home.find(b => b['@type'] === 'SoftwareApplication')!;
  const website = home.find(b => b['@type'] === 'WebSite')!;
  assert.deepEqual(software.author, { '@id': ORGANIZATION });
  assert.deepEqual(software.publisher, { '@id': ORGANIZATION });
  assert.deepEqual(website.publisher, { '@id': ORGANIZATION });
  assert.deepEqual(articuloDe(guiaJsonLd(guias[0]!))!.author, { '@id': ORGANIZATION });
  assert.deepEqual(articuloDe(guiaJsonLd(guias[0]!))!.publisher, { '@id': ORGANIZATION });
  assert.deepEqual(articuloDe(tutorialJsonLd(tutoriales[0]!))!.publisher, { '@id': ORGANIZATION });
});

test('the software and the website carry their own @id, and a page is part of the website by reference', () => {
  const home = parsear(homeJsonLd);
  assert.equal(home.find(b => b['@type'] === 'SoftwareApplication')!['@id'], SOFTWARE);
  assert.equal(home.find(b => b['@type'] === 'WebSite')!['@id'], WEBSITE);
  assert.deepEqual(parsear(privacidadJsonLd)[0]!.isPartOf, { '@id': WEBSITE });
  assert.deepEqual(articuloDe(guiaJsonLd(guias[0]!))!.isPartOf, { '@id': WEBSITE });
});

test('the home dateModified is the home page own date, not the repo-wide commit date', () => {
  const home = parsear(homeJsonLd).find(b => b['@type'] === 'SoftwareApplication')!;
  assert.equal(home.dateModified, PAGE_DATES['/']!.date);
});

test('the privacy and terms pages carry their own page dates', () => {
  assert.equal(parsear(privacidadJsonLd)[0]!.dateModified, PAGE_DATES['/privacidad']!.date);
  assert.equal(parsear(terminosJsonLd)[0]!.dateModified, PAGE_DATES['/terminos']!.date);
});

test('the guides index is dated by its own page, not by the repo-wide commit date', () => {
  const coleccion = parsear(guiasIndexJsonLd(guias))[0]!;
  assert.equal(coleccion.dateModified, PAGE_DATES['/guias']!.date);
});

test('the home datePublished is the day the home page first existed, not the 2026-01-01 placeholder', () => {
  const home = parsear(homeJsonLd).find(b => b['@type'] === 'SoftwareApplication')!;
  assert.equal(home.datePublished, PUBLISHED);
  assert.notEqual(PUBLISHED, '2026-01-01');
});
