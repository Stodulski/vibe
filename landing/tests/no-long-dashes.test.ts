/* El texto que ve la gente nunca lleva guion largo (— o –). Run with `pnpm test:unit`. */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { guias } from '../src/data/guias.ts';
import { tutoriales } from '../src/data/tutoriales.ts';
import * as competencia from '../src/data/competencia.ts';
import * as faq from '../src/data/faq.ts';
import * as precio from '../src/data/precio.ts';

const LONG_DASH = /[–—]/;

const sources: Record<string, unknown> = { guias, tutoriales, competencia, faq, precio };

for (const [name, data] of Object.entries(sources)) {
  test(`${name}: copy has no em or en dash`, () => {
    const text = JSON.stringify(data, (_key, value: unknown) => (typeof value === 'function' ? value.toString() : value));
    const found = LONG_DASH.exec(text);
    assert.equal(found, null, `dash near: ${found ? text.slice(Math.max(0, found.index - 50), found.index + 50) : ''}`);
  });
}
