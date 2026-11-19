/**
 * The story picture endpoint's answer: malformed requests and stories with nothing true to show
 * are uncached 404s, so the publisher never posts them; a drawn story is a JPEG cached for a
 * minute; API failures keep the share image's mapping. Reads and render are stubs.
 * Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError } from '../src/lib/api.ts';
import { storyImageResponse, type StoryImageDeps } from '../src/lib/story/story-response.ts';
import type { StoryAvailability } from '../src/lib/story/story-text.ts';

const TODAY = '2026-11-19';
const facts = { name: 'La Zona F5', city: 'Caleta Olivia', province: 'Santa Cruz', sports: ['Pádel'] };

function grid(available: boolean): StoryAvailability {
  return {
    date: TODAY,
    is_open: true,
    courts: [
      {
        court_id: 'c1',
        court_name: 'Cancha 1',
        sport: 'padel',
        slots: [{ start_time: '19:30', start_min: 1170, available }],
      },
    ],
  };
}

function deps(overrides: Partial<StoryImageDeps> = {}): StoryImageDeps & { renders: number } {
  const state = {
    renders: 0,
    readFacts: async () => facts,
    readAvailability: async () => grid(true),
    render: async () => {
      state.renders += 1;
      return new Uint8Array([0xff, 0xd8, 0xff]).buffer;
    },
    today: TODAY,
    labels: { padel: 'Pádel' },
    ...overrides,
  };
  return state;
}

test('a malformed request answers 404 without reading anything', async () => {
  let reads = 0;
  const response = await storyImageResponse('la-zona-f5', 'liberado', new URLSearchParams('hora=19:30'), {
    ...deps(),
    readFacts: async () => {
      reads += 1;
      return facts;
    },
  });
  assert.equal(response.status, 404);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(reads, 0);
});

test('a drawn story is a JPEG cached for a minute', async () => {
  const response = await storyImageResponse('la-zona-f5', 'libres', new URLSearchParams(), deps());
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Content-Type'), 'image/jpeg');
  assert.equal(response.headers.get('Cache-Control'), 'public, max-age=60, s-maxage=60');
});

test('a freed slot taken again answers an uncached 404 and draws nothing', async () => {
  const d = deps({ readAvailability: async () => grid(false) });
  const response = await storyImageResponse('la-zona-f5', 'liberado', new URLSearchParams('hora=19:30&cancha=c1'), d);
  assert.equal(response.status, 404);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(d.renders, 0);
});

test('API failures map to 404, 410 and a retryable 503', async () => {
  const fail = (kind: 'not_found' | 'gone' | 'unavailable') =>
    storyImageResponse(
      'la-zona-f5',
      'libres',
      new URLSearchParams(),
      deps({
        readAvailability: async () => {
          throw new ApiError(kind, kind === 'gone' ? 410 : 503);
        },
      }),
    );
  assert.equal((await fail('not_found')).status, 404);
  assert.equal((await fail('gone')).status, 410);
  const down = await fail('unavailable');
  assert.equal(down.status, 503);
  assert.equal(down.headers.get('Retry-After'), '60');
});
