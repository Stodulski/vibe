/**
 * The /og/c share image answer keeps exactly its headers: the PNG type and the day-long cache,
 * with no download header, now that the kit can add headers to the same answer. Run with
 * `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { complexImageResponse } from '../src/lib/og/complex-image.ts';

const facts = { name: 'La Zona F5', city: 'Caleta Olivia', province: 'Santa Cruz', sports: ['Pádel'] };

test('a complex image answers only the PNG type and the cache headers when no header is added', async () => {
  const response = await complexImageResponse('la-zona-f5', {
    readFacts: async () => facts,
    render: async () => new Uint8Array([0x89, 0x50, 0x4e, 0x47]).buffer,
  });
  assert.equal(response.status, 200);
  assert.deepEqual(Object.fromEntries(response.headers), {
    'cache-control': 'public, s-maxage=86400, stale-while-revalidate=604800',
    'content-type': 'image/png',
  });
});

test('a caller can add headers to the PNG answer, and they do not replace the cache headers', async () => {
  const response = await complexImageResponse(
    'la-zona-f5',
    {
      readFacts: async () => facts,
      render: async () => new Uint8Array([0x89, 0x50, 0x4e, 0x47]).buffer,
    },
    { headers: { 'Content-Disposition': 'attachment; filename="x.png"' } },
  );
  assert.equal(response.headers.get('Content-Disposition'), 'attachment; filename="x.png"');
  assert.equal(response.headers.get('Cache-Control'), 'public, s-maxage=86400, stale-while-revalidate=604800');
});
