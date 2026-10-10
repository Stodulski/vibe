/**
 * The storefront sitemap: the URL filter, the answer for each state of the switch and
 * the backend, and the API URL it reads. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createPublicApi } from '../src/lib/api.ts';
import {
  filterStorefrontSitemap,
  InvalidSitemap,
  isStorefrontLoc,
  storefrontSitemapAnswer,
} from '../src/lib/sitemap.ts';

const entry = (loc: string) =>
  `  <url>\n    <loc>${loc}</loc>\n    <lastmod>2026-10-01</lastmod>\n    <changefreq>daily</changefreq>\n    <priority>0.8</priority>\n  </url>\n`;

const backend = (...locs: string[]) =>
  `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${locs.map(entry).join('')}</urlset>\n`;

test('complex pages and city hubs on the storefront origin are kept', () => {
  assert.equal(isStorefrontLoc('https://vibe.com.ar/c/club-sur'), true);
  assert.equal(isStorefrontLoc('https://vibe.com.ar/canchas/banfield'), true);
});

test('every other address is dropped', () => {
  for (const loc of [
    'https://app.vibe.com.ar/c/club-sur',
    'https://vibe.com.ar/',
    'https://vibe.com.ar/c/',
    'https://vibe.com.ar/canchas',
    'https://vibe.com.ar/c/club/extra',
    'https://vibe.com.ar/c/club?x=1',
    'https://vibe.com.ar/c/club#x',
    'https://vibe.com.ar.evil.example/c/club',
    'http://vibe.com.ar/c/club',
    'https://vibe.com.ar/guias/algo',
    '',
  ]) {
    assert.equal(isStorefrontLoc(loc), false, loc);
  }
});

test('the filter keeps the storefront entries with their metadata and reports the rest', () => {
  const { xml, kept, dropped } = filterStorefrontSitemap(
    backend(
      'https://app.vibe.com.ar/c/club-sur',
      'https://vibe.com.ar/c/club-sur',
      'https://vibe.com.ar/canchas/banfield',
      'https://vibe.com.ar/',
    ),
  );
  assert.deepEqual(kept, ['https://vibe.com.ar/c/club-sur', 'https://vibe.com.ar/canchas/banfield']);
  assert.deepEqual(dropped, ['https://app.vibe.com.ar/c/club-sur', 'https://vibe.com.ar/']);
  assert.ok(xml.startsWith('<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n'));
  assert.equal((xml.match(/<url>/g) ?? []).length, 2);
  assert.ok(xml.includes('<lastmod>2026-10-01</lastmod>'));
  assert.equal(xml.includes('app.vibe.com.ar'), false);
  assert.ok(xml.endsWith('</urlset>\n'));
});

test('a backend answer that is not a complete urlset is refused', () => {
  assert.throws(() => filterStorefrontSitemap(''), InvalidSitemap);
  assert.throws(() => filterStorefrontSitemap('<html><body>502</body></html>'), InvalidSitemap);
  assert.throws(() => filterStorefrontSitemap(backend('https://vibe.com.ar/c/a').slice(0, -20)), InvalidSitemap);
});

test('a complete urlset with no storefront entries is a valid, empty answer', () => {
  const { xml, kept, dropped } = filterStorefrontSitemap(backend('https://app.vibe.com.ar/c/a'));
  assert.deepEqual(kept, []);
  assert.equal(dropped.length, 1);
  assert.equal(xml.includes('<url>'), false);
});

test('404 while the switch is off, and the API is not read', async () => {
  let read = false;
  const answer = await storefrontSitemapAnswer({
    indexable: false,
    load: async () => {
      read = true;
      return backend('https://vibe.com.ar/c/a');
    },
  });
  assert.equal(answer.status, 404);
  assert.equal(answer.headers['Cache-Control'], 'no-store');
  assert.equal(read, false);
});

test('503 with Retry-After when the backend read fails, never a stale or empty 200', async () => {
  const reports: string[] = [];
  const failures: Array<() => Promise<string>> = [
    async () => {
      throw new TypeError('fetch failed');
    },
    async () => '<html><body>502</body></html>',
    async () => '',
  ];
  for (const load of failures) {
    const answer = await storefrontSitemapAnswer({ indexable: true, load, report: (m) => reports.push(m) });
    assert.equal(answer.status, 503);
    assert.equal(answer.headers['Retry-After'], '300');
    assert.equal(answer.headers['Cache-Control'], 'no-store');
  }
  assert.equal(reports.length, failures.length);
});

test('an API 502 reaches the answer as 503', async () => {
  const api = createPublicApi({
    baseUrl: 'https://api.test/api/v1',
    fetchImpl: (async () => new Response('bad gateway', { status: 502 })) as typeof fetch,
  });
  const answer = await storefrontSitemapAnswer({ indexable: true, load: () => api.sitemapXml() });
  assert.equal(answer.status, 503);
});

test('200 with only the storefront URLs, cacheable at the edge, and the dropped ones reported', async () => {
  const reports: Array<[string, unknown]> = [];
  const answer = await storefrontSitemapAnswer({
    indexable: true,
    load: async () => backend('https://app.vibe.com.ar/c/a', 'https://vibe.com.ar/c/a'),
    report: (message, detail) => reports.push([message, detail]),
  });
  assert.equal(answer.status, 200);
  assert.equal(answer.headers['Content-Type'], 'application/xml; charset=utf-8');
  assert.equal(answer.headers['Cache-Control'], 'public, s-maxage=3600, stale-while-revalidate=600');
  assert.equal(answer.body.includes('https://vibe.com.ar/c/a'), true);
  assert.equal(answer.body.includes('app.vibe.com.ar'), false);
  assert.deepEqual(reports, [['storefront sitemap dropped URLs outside the storefront prefixes', ['https://app.vibe.com.ar/c/a']]]);
});

test('the sitemap request goes to the API root under /sitemap.xml', async () => {
  const calls: string[] = [];
  const fetchImpl = (async (input: string | URL | Request) => {
    calls.push(String(input));
    return new Response(backend(), { headers: { 'Content-Type': 'application/xml' } });
  }) as typeof fetch;
  const api = createPublicApi({ baseUrl: 'https://api.test/api/v1/', fetchImpl });
  await api.sitemapXml();
  assert.deepEqual(calls, ['https://api.test/api/v1/sitemap.xml']);
});
