/**
 * The kit picture endpoint's answer: an unknown piece or format is a 404 before any read, the
 * download header comes only with ?descargar=1, and the failures are the share image's own. The
 * API read and the render are stubs. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError } from '../src/lib/api.ts';
import { kitImageResponse, type KitImageDeps } from '../src/lib/kit/kit-response.ts';

const facts = { name: 'La Zona F5', city: 'Caleta Olivia', province: 'Santa Cruz', sports: ['Pádel'] };
const CACHE = 'public, s-maxage=86400, stale-while-revalidate=604800';

function pngStub(): ArrayBuffer {
  return new Uint8Array([0x89, 0x50, 0x4e, 0x47]).buffer;
}

function stubs(overrides: Partial<KitImageDeps> = {}): KitImageDeps {
  return {
    readFacts: async () => facts,
    render: async () => pngStub(),
    ...overrides,
  };
}

test('an unknown piece or format answers 404 without reading the complex or rendering', async () => {
  let reads = 0;
  const response = await kitImageResponse('la-zona-f5', 'reservar-cuadrado', new URLSearchParams(), {
    readFacts: async () => {
      reads += 1;
      return facts;
    },
    render: async () => {
      throw new Error('must not render');
    },
  });
  assert.equal(response.status, 404);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(reads, 0);
});

test('a kit picture answers the PNG with the share image cache headers and no download header', async () => {
  const response = await kitImageResponse('la-zona-f5', 'reservar-historia', new URLSearchParams(), stubs());
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Content-Type'), 'image/png');
  assert.equal(response.headers.get('Cache-Control'), CACHE);
  assert.equal(response.headers.get('Content-Disposition'), null);
});

test('?descargar=1 adds the attachment name with the slug, the piece and the format', async () => {
  const response = await kitImageResponse(
    'la-zona-f5',
    'como-reservar-post',
    new URLSearchParams('descargar=1'),
    stubs(),
  );
  assert.equal(response.status, 200);
  assert.equal(
    response.headers.get('Content-Disposition'),
    'attachment; filename="vibe-la-zona-f5-como-reservar-post.png"',
  );
  assert.equal(response.headers.get('Cache-Control'), CACHE);
});

test('a download request without the flag value 1 gets no attachment name', async () => {
  const response = await kitImageResponse('la-zona-f5', 'reservar-post', new URLSearchParams('descargar=0'), stubs());
  assert.equal(response.headers.get('Content-Disposition'), null);
});

test('the render receives the complex facts, the slug and the picture it draws', async () => {
  let received: unknown = null;
  await kitImageResponse(
    'la-zona-f5',
    'reservar-post',
    new URLSearchParams(),
    stubs({
      render: async (card, slug, image) => {
        received = { card, slug, image };
        return pngStub();
      },
    }),
  );
  assert.deepEqual(received, { card: facts, slug: 'la-zona-f5', image: { piece: 'reservar', format: 'post' } });
});

test('a complex the API does not know answers 404, and a deleted one 410, neither cached', async () => {
  const missing = await kitImageResponse(
    'no-existe',
    'reservar-historia',
    new URLSearchParams('descargar=1'),
    stubs({
      readFacts: async () => {
        throw new ApiError('not_found', 404);
      },
    }),
  );
  assert.equal(missing.status, 404);
  assert.equal(missing.headers.get('Cache-Control'), 'no-store');
  assert.equal(missing.headers.get('Content-Disposition'), null);

  const gone = await kitImageResponse(
    'viejo',
    'reservar-historia',
    new URLSearchParams(),
    stubs({
      readFacts: async () => {
        throw new ApiError('gone', 410, { gone: { name: 'Viejo', city: 'Rosario', citySlug: 'rosario' } });
      },
    }),
  );
  assert.equal(gone.status, 410);
  assert.equal(gone.headers.get('Cache-Control'), 'no-store');
});

test('a failed read answers 503 with Retry-After and a render that throws answers 500, neither cached', async () => {
  const unavailable = await kitImageResponse(
    'la-zona-f5',
    'reservar-historia',
    new URLSearchParams(),
    stubs({
      readFacts: async () => {
        throw new ApiError('unavailable', null);
      },
    }),
  );
  assert.equal(unavailable.status, 503);
  assert.equal(unavailable.headers.get('Retry-After'), '60');
  assert.equal(unavailable.headers.get('Cache-Control'), 'no-store');

  const broken = await kitImageResponse(
    'la-zona-f5',
    'reservar-historia',
    new URLSearchParams(),
    stubs({
      render: async () => {
        throw new Error('wasm failed');
      },
    }),
  );
  assert.equal(broken.status, 500);
  assert.equal(broken.headers.get('Cache-Control'), 'no-store');
});
