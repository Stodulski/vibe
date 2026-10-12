/**
 * llms.txt: the llmstxt.org index /llms.txt answers on every request. The renderer is pure; the
 * endpoint feeds it the static part (src/data/llms-static.json) and, when the API answers, the
 * city hubs and complexes read through the public API client. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createPublicApi, type PublicApi } from '../src/lib/api.ts';
import { llmsTxt, renderLlmsTxt, type LlmsStatic } from '../src/lib/markdown/llms-txt.ts';

const SITE = 'https://vibe.com.ar';
const base: LlmsStatic = {
  title: 'Vibe',
  description: 'Sistema de gestión gratis para complejos deportivos.',
  summary: 'Vibe reúne reservas, cobros y caja.',
  updated: '2026-10-11',
  pages: [{ title: 'Guías de Vibe', url: `${SITE}/guias`, md: `${SITE}/guias.md`, note: 'Respuestas con fuente.' }],
};

test('starts with the H1, the summary blockquote and the site summary', () => {
  const out = renderLlmsTxt({ ...base, cities: [], complexes: [] });
  const lines = out.split('\n');
  assert.equal(lines[0], '# Vibe');
  assert.equal(lines[2], `> ${base.description}`);
  assert.ok(out.includes(base.summary));
});

test('every entry is a linked bullet with a note and its Markdown twin', () => {
  const out = renderLlmsTxt({ ...base, cities: [], complexes: [] });
  assert.ok(out.includes(`- [Guías de Vibe](${SITE}/guias): Respuestas con fuente. — Markdown: ${SITE}/guias.md`));
});

test('the sections for cities and complexes appear only when there is something to list', () => {
  const staticOnly = renderLlmsTxt({ ...base, cities: [], complexes: [] });
  assert.equal(staticOnly.includes('## Canchas'), false);
  assert.equal(staticOnly.includes('## Complejos'), false);

  const full = renderLlmsTxt({
    ...base,
    cities: [{ title: 'Canchas en Rosario', url: `${SITE}/canchas/rosario`, md: `${SITE}/canchas-rosario.md`, note: 'Santa Fe · 2 complejos' }],
    complexes: [{ title: 'La Zona F5', url: `${SITE}/c/la-zona-f5`, md: `${SITE}/c-la-zona-f5.md`, note: 'Piquiyin, Caleta Olivia' }],
  });
  assert.ok(full.includes('## Canchas\n'));
  assert.ok(full.includes('## Complejos\n'));
  assert.ok(full.includes(`- [La Zona F5](${SITE}/c/la-zona-f5): Piquiyin, Caleta Olivia — Markdown: ${SITE}/c-la-zona-f5.md`));
});

test('the claim about Accept: text/markdown is written as a fact the site keeps', () => {
  const out = renderLlmsTxt({ ...base, cities: [], complexes: [] });
  assert.ok(out.includes('Accept: text/markdown'));
});

test('brackets in a title cannot break the link syntax', () => {
  const out = renderLlmsTxt({
    ...base,
    cities: [],
    complexes: [{ title: 'Club [Norte]', url: `${SITE}/c/club-norte`, md: `${SITE}/c-club-norte.md`, note: 'Rosario' }],
  });
  assert.ok(out.includes(`- [Club (Norte)](${SITE}/c/club-norte): Rosario`));
});

/* The composition: the endpoint's answer. The API is the real client, with a fake transport. */

const HUBS = {
  hubs: [
    { slug: 'rosario', name: 'Rosario', province: 'Santa Fe', complex_count: 1, sports: ['padel'] },
    { slug: 'cordoba', name: 'Córdoba', province: 'Córdoba', complex_count: 1, sports: ['tenis'] },
  ],
};
const DETAIL_ROSARIO = {
  hub: { slug: 'rosario', name: 'Rosario', province: 'Santa Fe' },
  complexes: [{ slug: 'la-zona-f5', name: 'La Zona F5', address: 'Piquiyin 123' }],
  other_hubs: [],
};
const DETAIL_CORDOBA = {
  hub: { slug: 'cordoba', name: 'Córdoba', province: 'Córdoba' },
  complexes: [{ slug: 'cordoba-centro', name: 'Centro', address: 'Alem 1' }],
  other_hubs: [],
};

/** An API that answers from a table of payloads by path; a path missing from it is a 404. */
function apiServing(table: Record<string, { status?: number; body?: unknown }>): PublicApi {
  return createPublicApi({
    baseUrl: 'https://api.test/api/v1',
    fetchImpl: (async (input: string | URL | Request) => {
      const path = new URL(String(input)).pathname.replace('/api/v1', '');
      const hit = table[path];
      if (!hit) return new Response('', { status: 404 });
      return new Response(JSON.stringify(hit.body ?? {}), {
        status: hit.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }) as typeof fetch,
  });
}

/** An API that cannot be reached at all. */
const apiDown = (): PublicApi =>
  createPublicApi({
    baseUrl: 'https://api.test/api/v1',
    fetchImpl: (async () => {
      throw new TypeError('fetch failed');
    }) as typeof fetch,
  });

test('when the API answers, the cities and complexes are listed after the static pages', async () => {
  const api = apiServing({
    '/public/hubs': { body: HUBS },
    '/public/hubs/rosario/data': { body: DETAIL_ROSARIO },
    '/public/hubs/cordoba/data': { body: DETAIL_CORDOBA },
  });
  const reports: string[] = [];
  const out = await llmsTxt(base, api, (message) => reports.push(message));

  assert.deepEqual(reports, []);
  assert.ok(out.includes(`- [Canchas en Vibe](${SITE}/canchas): Todas las ciudades con complejos — Markdown: ${SITE}/canchas.md`));
  assert.ok(out.includes(`- [Canchas en Rosario](${SITE}/canchas/rosario): Santa Fe · 1 complejo(s) — Markdown: ${SITE}/canchas-rosario.md`));
  assert.ok(out.includes(`- [La Zona F5](${SITE}/c/la-zona-f5): Piquiyin 123, Rosario — Markdown: ${SITE}/c-la-zona-f5.md`));
  const order = ['## Páginas', '## Canchas', '## Complejos'].map((heading) => out.indexOf(heading));
  assert.ok(order.every((at) => at > -1) && order[0] < order[1] && order[1] < order[2], 'sections in order');
});

test('an API that cannot be reached still answers with the static sections only, and reports why', async () => {
  const reports: string[] = [];
  const out = await llmsTxt(base, apiDown(), (message) => reports.push(message));

  assert.equal(out, renderLlmsTxt({ ...base, cities: [], complexes: [] }));
  assert.equal(out.includes('## Canchas'), false);
  assert.equal(out.includes('## Complejos'), false);
  assert.ok(out.includes('## Páginas'));
  assert.equal(reports.length, 1);
});

test('a city whose details fail keeps its own entry and loses only its complexes', async () => {
  const api = apiServing({
    '/public/hubs': { body: HUBS },
    '/public/hubs/rosario/data': { body: DETAIL_ROSARIO },
    '/public/hubs/cordoba/data': { status: 500 },
  });
  const reports: string[] = [];
  const out = await llmsTxt(base, api, (message) => reports.push(message));

  assert.ok(out.includes(`- [Canchas en Córdoba](${SITE}/canchas/cordoba)`));
  assert.equal(out.includes('Centro'), false);
  assert.ok(out.includes('La Zona F5'));
  assert.equal(reports.length, 1);
});
