/**
 * The share image endpoint's answer: a PNG for a complex the API reads, and an answer no
 * crawler can cache as the image when the read fails. The render is the real satori and
 * resvg pipeline; only the API read is stubbed. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { ApiError } from '../src/lib/api.ts';
import { complexImageResponse } from '../src/lib/og/complex-image.ts';
import { renderComplexCard, type CardAssets } from '../src/lib/og/render.ts';

function bytesOf(path: URL): ArrayBuffer {
  const buffer = readFileSync(path);
  return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);
}

const assets: CardAssets = {
  regular: bytesOf(new URL('../node_modules/@fontsource/dm-sans/files/dm-sans-latin-400-normal.woff', import.meta.url)),
  bold: bytesOf(new URL('../node_modules/@fontsource/dm-sans/files/dm-sans-latin-700-normal.woff', import.meta.url)),
  wasm: bytesOf(new URL('../node_modules/@resvg/resvg-wasm/index_bg.wasm', import.meta.url)),
};

const facts = {
  name: 'La Zona F5',
  city: 'Caleta Olivia',
  province: 'Santa Cruz',
  sports: ['Pádel', 'Fútbol'],
};

const PNG_SIGNATURE = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];

test('a complex the API reads answers a PNG with the cache headers the share image needs', async () => {
  const response = await complexImageResponse('la-zona-f5', {
    readFacts: async () => facts,
    render: (card) => renderComplexCard(card, assets),
  });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Content-Type'), 'image/png');
  assert.equal(response.headers.get('Cache-Control'), 'public, s-maxage=86400, stale-while-revalidate=604800');
  const bytes = new Uint8Array(await response.arrayBuffer());
  assert.deepEqual([...bytes.slice(0, 8)], PNG_SIGNATURE);
});

test('a complex the API does not know answers 404 and no crawler caches it as the image', async () => {
  const response = await complexImageResponse('no-existe', {
    readFacts: async () => {
      throw new ApiError('not_found', 404);
    },
    render: () => {
      throw new Error('must not render');
    },
  });
  assert.equal(response.status, 404);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.notEqual(response.headers.get('Content-Type'), 'image/png');
});

test('a deleted complex answers 410 and is not cached as an image', async () => {
  const response = await complexImageResponse('viejo', {
    readFacts: async () => {
      throw new ApiError('gone', 410, { gone: { name: 'Viejo', city: 'Rosario', citySlug: 'rosario' } });
    },
    render: () => {
      throw new Error('must not render');
    },
  });
  assert.equal(response.status, 410);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
});

test('a failed read answers 503 with Retry-After and is not cached as the image', async () => {
  const response = await complexImageResponse('la-zona-f5', {
    readFacts: async () => {
      throw new ApiError('unavailable', null);
    },
    render: () => {
      throw new Error('must not render');
    },
  });
  assert.equal(response.status, 503);
  assert.equal(response.headers.get('Retry-After'), '60');
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
});

test('a render that throws answers 500 and is not cached as the image', async () => {
  const response = await complexImageResponse('la-zona-f5', {
    readFacts: async () => facts,
    render: async () => {
      throw new Error('wasm failed');
    },
  });
  assert.equal(response.status, 500);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
});
