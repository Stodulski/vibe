/* Tutorial steps: a numbered step with a title, a text and a desktop capture that already
   carries its markers. The data, the JSON-LD and the built page plus its Markdown twin
   must all say the same steps. Run with `pnpm test:unit`; the built-output tests need
   `pnpm build` first and are skipped without dist/. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { tutoriales, getTutorial, getRelacionados } from '../src/data/tutoriales.ts';
import { tutorialJsonLd } from '../src/data/jsonld.ts';
import type { Bloque } from '../src/data/guias.ts';

const SLUG = 'plazo-de-acreditacion-mercado-pago';
const PUBLIC = new URL('../public/', import.meta.url);
const DIST = new URL('../dist/', import.meta.url);
const sinBuild = !existsSync(DIST) && 'needs `pnpm build` first';

type Pasos = Extract<Bloque, { tipo: 'pasos' }>;
const pasosDe = (bloques: Bloque[]) => bloques.filter((b): b is Pasos => b.tipo === 'pasos').flatMap(b => b.pasos);

/** Width and height read from the file header, PNG or WebP. */
function medidas(file: URL): [number, number] {
  const b = readFileSync(file);
  if (b.toString('ascii', 1, 4) === 'PNG') return [b.readUInt32BE(16), b.readUInt32BE(20)];
  const chunk = b.toString('ascii', 12, 16);
  if (chunk === 'VP8X') return [1 + b.readUIntLE(24, 3), 1 + b.readUIntLE(27, 3)];
  if (chunk === 'VP8L') {
    const bits = b.readUInt32LE(21);
    return [1 + (bits & 0x3fff), 1 + ((bits >> 14) & 0x3fff)];
  }
  return [b.readUInt16LE(26) & 0x3fff, b.readUInt16LE(28) & 0x3fff];
}

test('the Mercado Pago release period tutorial exists and points at its guide and related tutorials', () => {
  const t = getTutorial(SLUG);
  assert.ok(t, 'missing tutorial');
  assert.equal(t.guia?.slug, 'cuanto-cobra-mercadopago-por-una-sena');
  assert.ok(t.relacionados.includes('configuracion-del-complejo'));
  assert.equal(getRelacionados(t).length, t.relacionados.length, 'a related slug does not resolve');
});

test('every step has a title, a text, an alt and a capture whose files exist at the declared size', () => {
  const pasos = pasosDe(getTutorial(SLUG)?.bloques ?? []);
  assert.equal(pasos.length, 6); // one step per marker in the captures
  for (const p of pasosDe(tutoriales.flatMap(t => t.bloques))) {
    assert.ok(p.titulo.trim() && p.texto.trim() && p.imagen.alt.trim(), p.titulo);
    assert.match(p.imagen.src, /^\/tutoriales\/pasos\/[a-z0-9-]+\/[a-z0-9-]+$/);
    for (const ext of ['webp', 'png']) {
      const file = new URL(`.${p.imagen.src}.${ext}`, PUBLIC);
      assert.ok(existsSync(file), `missing ${file.pathname}`);
      assert.deepEqual(medidas(file), [p.imagen.width, p.imagen.height], file.pathname);
    }
    const chica = new URL(`.${p.imagen.src}-960.webp`, PUBLIC);
    assert.ok(existsSync(chica), `missing ${chica.pathname}`);
  }
});

test('the tutorial copy states no price or commission as a number and has no exclamation marks', () => {
  const texto = JSON.stringify(getTutorial(SLUG));
  assert.doesNotMatch(texto, /\d\s*%|\$\s*\d|[!¡]/);
  assert.doesNotMatch(texto, /MercadoPago/, 'the brand is spelled "Mercado Pago" in new copy');
});

test('a tutorial with steps lists them in its Article, numbered, each with its absolute capture', () => {
  const t = getTutorial(SLUG)!;
  const articulo = tutorialJsonLd(t).map(b => JSON.parse(b)).find(b => b['@type'] === 'Article');
  const lista = articulo?.hasPart;
  assert.equal(lista?.['@type'], 'ItemList');
  const pasos = pasosDe(t.bloques);
  assert.deepEqual(
    lista.itemListElement.map((s: Record<string, unknown>) => [s['@type'], s.position, s.name, s.description, s.image]),
    pasos.map((p, i) => ['ListItem', i + 1, p.titulo, p.texto, `https://vibe.com.ar${p.imagen.src}.webp`]),
  );
});

test('no tutorial ships a HowTo (Google retired it) and one without steps has no step list', () => {
  for (const t of tutoriales) {
    assert.equal(tutorialJsonLd(t).some(b => JSON.parse(b)['@type'] === 'HowTo'), false, t.slug);
  }
  const sinPasos = tutoriales.find(t => pasosDe(t.bloques).length === 0)!;
  const articulo = tutorialJsonLd(sinPasos).map(b => JSON.parse(b)).find(b => b['@type'] === 'Article');
  assert.equal(articulo.hasPart, undefined);
});

test('each step of the Mercado Pago tutorial zooms into its markers with a crop on disk', () => {
  for (const p of pasosDe(getTutorial(SLUG)!.bloques)) {
    const r = p.imagen.recorte;
    assert.ok(r, `step without crop: ${p.titulo}`);
    assert.equal(r.src.slice(0, r.src.lastIndexOf('/')), p.imagen.src.slice(0, p.imagen.src.lastIndexOf('/')));
    assert.ok(r.width < p.imagen.width && r.height <= p.imagen.height, 'the crop is smaller than the capture');
    for (const ext of ['png', 'webp']) {
      assert.deepEqual(medidas(new URL(`.${r.src}.${ext}`, PUBLIC)), [r.width, r.height], `${r.src}.${ext}`);
    }
  }
});

test('the built page renders each step with its number, title, text and a lazy sized capture', { skip: sinBuild }, () => {
  const html = readFileSync(new URL(`tutoriales/${SLUG}/index.html`, DIST), 'utf8');
  const pasos = pasosDe(getTutorial(SLUG)!.bloques);
  assert.equal(html.match(/class="tutorial-paso"/g)?.length, pasos.length);
  pasos.forEach((p, i) => {
    assert.ok(html.includes(`Paso ${i + 1}`), `Paso ${i + 1}`);
    assert.ok(html.includes(p.titulo), p.titulo);
    // The page shows the zoomed crop around the step's markers; a tap opens the whole capture.
    const vista = p.imagen.recorte ?? p.imagen;
    const img = html.match(new RegExp(`<img[^>]*src="${vista.src}\\.png"[^>]*>`))?.[0];
    assert.ok(img, `missing img for ${vista.src}`);
    assert.match(img, /loading="lazy"/);
    assert.match(img, new RegExp(`width="${vista.width}"`));
    assert.match(img, new RegExp(`height="${vista.height}"`));
    assert.ok(html.includes(`href="${p.imagen.src}.webp"`), 'missing link to the whole capture');
    if (!p.imagen.recorte) assert.ok(html.includes(`${p.imagen.src}-960.webp 960w`), 'missing small WebP in srcset');
  });
});

test('the Markdown twin lists each step as a heading with its text and absolute capture', { skip: sinBuild }, () => {
  const md = readFileSync(new URL(`tutoriales-${SLUG}.md`, DIST), 'utf8');
  pasosDe(getTutorial(SLUG)!.bloques).forEach((p, i) => {
    assert.ok(md.includes(`### Paso ${i + 1}. ${p.titulo}`), `heading of step ${i + 1}`);
    assert.ok(md.includes(p.texto), `text of step ${i + 1}`);
    assert.ok(md.includes(`![${p.imagen.alt}](https://vibe.com.ar${p.imagen.src}.webp)`), `capture of step ${i + 1}`);
  });
});

test('the tutorials index lists the new tutorial', { skip: sinBuild }, () => {
  const html = readFileSync(new URL('tutoriales/index.html', DIST), 'utf8');
  assert.ok(html.includes(`href="/tutoriales/${SLUG}"`));
});
