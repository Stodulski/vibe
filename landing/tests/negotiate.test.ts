/**
 * Content negotiation for agents: which requests get the Markdown twin, and the flat file
 * name each route maps to. The Vercel middleware (landing/middleware.ts) calls these; the
 * matcher it runs on is checked against the real page files here. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { MARKDOWN_MATCHER, markdownTwinPath, matcherCovers, wantsMarkdown } from '../src/lib/markdown/negotiate.ts';

const LANDING = fileURLToPath(new URL('..', import.meta.url));

test('an agent that names text/markdown gets the twin', () => {
  assert.equal(wantsMarkdown('text/markdown'), true);
  assert.equal(wantsMarkdown('text/markdown, text/html;q=0.5'), true);
  assert.equal(wantsMarkdown('text/markdown, */*;q=0.1'), true);
});

test('a browser that prefers HTML keeps HTML, even when it also lists markdown', () => {
  assert.equal(wantsMarkdown('text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8'), false);
  assert.equal(wantsMarkdown('text/html, text/markdown;q=0.5'), false);
});

test('no Accept header, a wildcard alone, or markdown refused with q=0 keep HTML', () => {
  assert.equal(wantsMarkdown(null), false);
  assert.equal(wantsMarkdown(undefined), false);
  assert.equal(wantsMarkdown(''), false);
  assert.equal(wantsMarkdown('*/*'), false);
  assert.equal(wantsMarkdown('text/markdown;q=0'), false);
});

test('the twin of a route is its flat file name, as build-markdown writes it', () => {
  assert.equal(markdownTwinPath('/'), '/index.md');
  assert.equal(markdownTwinPath('/privacidad'), '/privacidad.md');
  assert.equal(markdownTwinPath('/guias/algo'), '/guias-algo.md');
  assert.equal(markdownTwinPath('/tutoriales/abrir-y-cerrar-la-caja'), '/tutoriales-abrir-y-cerrar-la-caja.md');
  assert.equal(markdownTwinPath('/canchas'), '/canchas.md');
  assert.equal(markdownTwinPath('/canchas/rosario'), '/canchas-rosario.md');
  assert.equal(markdownTwinPath('/c/la-zona-f5'), '/c-la-zona-f5.md');
});

test('a trailing slash names the same twin', () => {
  assert.equal(markdownTwinPath('/canchas/'), '/canchas.md');
  assert.equal(markdownTwinPath('/guias/algo/'), '/guias-algo.md');
  assert.equal(markdownTwinPath('/'), '/index.md');
});

test('every public page route is answered by the Vercel matcher, so none stays HTML for agents', () => {
  const routes = publicPageRoutes();
  assert.ok(routes.length >= 10, `expected the public pages, found ${routes.length}`);
  const uncovered = routes.filter((route) => !matcherCovers(route));
  assert.deepEqual(uncovered, []);
});

test('the matcher covers the server-rendered storefront routes that the old list omitted', () => {
  for (const route of ['/canchas', '/canchas/rosario', '/c/la-zona-f5', '/tutoriales/abrir-y-cerrar-la-caja']) {
    assert.equal(matcherCovers(route), true, route);
  }
  // Booking steps are private and noindex, so they never negotiate.
  assert.equal(matcherCovers('/c/la-zona-f5/book'), false);
});

/**
 * The route each public page file serves, read from src/pages. Dynamic segments come back
 * as ":param" so they can be checked against the matcher's own syntax.
 */
function publicPageRoutes(): string[] {
  const pages = join(LANDING, 'src', 'pages');
  const routes: string[] = [];
  for (const file of walk(pages)) {
    const rel = relative(pages, file).replace(/\\/g, '/');
    if (!rel.endsWith('.astro')) continue;
    if (rel.startsWith('c/') && rel.includes('/book/')) continue; // booking steps: noindex
    if (rel === '404.astro' || rel.startsWith('c/') && !rel.endsWith('index.astro')) continue;
    const route = '/' + rel.replace(/\.astro$/, '').replace(/\/index$/, '').replace(/\[(\w+)\]/g, ':$1');
    routes.push(route === '/index' ? '/' : route);
  }
  return routes.sort();
}

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    return statSync(full).isDirectory() ? walk(full) : [full];
  });
}

test('the literal Vercel matcher lists exactly the patterns the negotiation module covers', () => {
  // Vercel reads `config.matcher` statically, so the middleware keeps a literal list;
  // this keeps that literal and the module from drifting apart.
  const source = readFileSync(join(LANDING, 'middleware.ts'), 'utf8');
  const literal = source.match(/matcher:\s*\[([^\]]*)\]/)?.[1] ?? '';
  const patterns = [...literal.matchAll(/'([^']+)'/g)].map((m) => m[1]);
  assert.deepEqual(patterns, MARKDOWN_MATCHER);
});
