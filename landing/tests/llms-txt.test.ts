/**
 * llms.txt: the llmstxt.org index the build writes to dist/. The renderer is pure; the build
 * feeds it the static pages and, when the API answers, the city hubs and complexes. Run with
 * `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
// @ts-expect-error the renderer is a build-time .mjs module shared with scripts/build-markdown.mjs
import { renderLlmsTxt } from '../scripts/llms-txt.mjs';

const SITE = 'https://vibe.com.ar';
const base = {
  title: 'Vibe',
  description: 'Sistema de gestión gratis para complejos deportivos.',
  summary: 'Vibe reúne reservas, cobros y caja.',
  updated: '2026-10-11',
  pages: [{ title: 'Guías de Vibe', url: `${SITE}/guias`, md: `${SITE}/guias.md`, note: 'Respuestas con fuente.' }],
  cities: [],
  complexes: [],
};

test('starts with the H1, the summary blockquote and the site summary', () => {
  const out = renderLlmsTxt(base) as string;
  const lines = out.split('\n');
  assert.equal(lines[0], '# Vibe');
  assert.equal(lines[2], `> ${base.description}`);
  assert.ok(out.includes(base.summary));
});

test('every entry is a linked bullet with a note and its Markdown twin', () => {
  const out = renderLlmsTxt(base) as string;
  assert.ok(out.includes(`- [Guías de Vibe](${SITE}/guias): Respuestas con fuente. — Markdown: ${SITE}/guias.md`));
});

test('the sections for cities and complexes appear only when there is something to list', () => {
  const staticOnly = renderLlmsTxt(base) as string;
  assert.equal(staticOnly.includes('## Canchas'), false);
  assert.equal(staticOnly.includes('## Complejos'), false);

  const full = renderLlmsTxt({
    ...base,
    cities: [{ title: 'Canchas en Rosario', url: `${SITE}/canchas/rosario`, md: `${SITE}/canchas-rosario.md`, note: 'Santa Fe · 2 complejos' }],
    complexes: [{ title: 'La Zona F5', url: `${SITE}/c/la-zona-f5`, md: `${SITE}/c-la-zona-f5.md`, note: 'Piquiyin, Caleta Olivia' }],
  }) as string;
  assert.ok(full.includes('## Canchas\n'));
  assert.ok(full.includes('## Complejos\n'));
  assert.ok(full.includes(`- [La Zona F5](${SITE}/c/la-zona-f5): Piquiyin, Caleta Olivia — Markdown: ${SITE}/c-la-zona-f5.md`));
});

test('the claim about Accept: text/markdown is written as a fact the site keeps', () => {
  const out = renderLlmsTxt(base) as string;
  assert.ok(out.includes('Accept: text/markdown'));
});

test('brackets in a title cannot break the link syntax', () => {
  const out = renderLlmsTxt({
    ...base,
    complexes: [{ title: 'Club [Norte]', url: `${SITE}/c/club-norte`, md: `${SITE}/c-club-norte.md`, note: 'Rosario' }],
  }) as string;
  assert.ok(out.includes(`- [Club (Norte)](${SITE}/c/club-norte): Rosario`));
});
