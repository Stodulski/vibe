import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { courtsApi } from './courts.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeCourt, makePrice } from '@/test/factories';

async function expectApiResponseError(promise: Promise<unknown>, context: string) {
  try {
    await promise;
    throw new Error('expected promise to reject');
  } catch (err) {
    expect(err).toBeInstanceOf(ApiResponseError);
    expect((err as ApiResponseError).context).toBe(context);
  }
}

describe('courtsApi response validation', () => {
  it('list resolves with a valid response', async () => {
    server.use(
      http.get('*/complexes/:complexId/courts', () =>
        HttpResponse.json({ courts: [{ ...makeCourt(), prices: [makePrice()] }] }),
      ),
    );
    const result = await courtsApi.list('c1');
    expect(result.courts).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when prices is missing', async () => {
    server.use(http.get('*/complexes/:complexId/courts', () => HttpResponse.json({ courts: [makeCourt()] })));
    await expectApiResponseError(courtsApi.list('c1'), 'courtsApi.list');
  });

  it('blockSlot rejects with ApiResponseError carrying its context when blocked_slot is missing', async () => {
    server.use(http.post('*/complexes/:complexId/courts/:courtId/block', () => HttpResponse.json({})));
    await expectApiResponseError(
      courtsApi.blockSlot('c1', 'ct1', {
        date: '2026-03-18',
        start_time: '10:00',
        end_time: '11:00',
      }),
      'courtsApi.blockSlot',
    );
  });
});
