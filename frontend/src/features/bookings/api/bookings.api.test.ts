// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { bookingsApi } from './bookings.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeBooking } from '@/test/factories';

async function expectApiResponseError(promise: Promise<unknown>, context: string) {
  try {
    await promise;
    throw new Error('expected promise to reject');
  } catch (err) {
    expect(err).toBeInstanceOf(ApiResponseError);
    expect((err as ApiResponseError).context).toBe(context);
  }
}

describe('bookingsApi response validation', () => {
  it('list resolves with a valid response', async () => {
    server.use(
      http.get('*/complexes/:complexId/bookings', () =>
        HttpResponse.json({ bookings: [makeBooking()], metadata: { has_more: false } }),
      ),
    );
    const result = await bookingsApi.list('c1', '2026-03-18');
    expect(result.bookings).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when deposit_amount is a string', async () => {
    server.use(
      http.get('*/complexes/:complexId/bookings', () =>
        HttpResponse.json({
          bookings: [{ ...makeBooking(), deposit_amount: '0' }],
          metadata: { has_more: false },
        }),
      ),
    );
    await expectApiResponseError(bookingsApi.list('c1', '2026-03-18'), 'bookingsApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when booking is missing', async () => {
    server.use(http.get('*/complexes/:complexId/bookings/:bookingId', () => HttpResponse.json({})));
    await expectApiResponseError(bookingsApi.getById('c1', 'b1'), 'bookingsApi.getById');
  });
});
