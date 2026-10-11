/* El <title> de una guia sale de un solo lugar, con el mismo sufijo que los
   tutoriales. Run with `pnpm test:unit`. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { guias, guiaTitle } from '../src/data/guias.ts';

/* La banda es la misma que aplica scripts/seo-gate.mjs al title renderizado. */
const MIN = 20;
const MAX = 60;

test('every guia <title> ends with " | Vibe", as the tutorials already do', () => {
  for (const g of guias) {
    assert.ok(guiaTitle(g).endsWith(' | Vibe'), `${g.slug}: ${guiaTitle(g)}`);
  }
});

test('every guia <title> stays inside the band seo-gate enforces', () => {
  for (const g of guias) {
    const largo = guiaTitle(g).length;
    assert.ok(largo >= MIN && largo <= MAX, `${g.slug} mide ${largo}: ${guiaTitle(g)}`);
  }
});
