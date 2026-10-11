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
