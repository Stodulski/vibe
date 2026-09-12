import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { HTTPError } from 'ky';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: {
    create: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/** Mutates once with a rejected `bookingsApi.create` and waits for the error state. */
async function triggerCreateError(backendError: HTTPError) {
  const { bookingsApi } = await import('../api/bookings.api');
  vi.mocked(bookingsApi.create).mockRejectedValueOnce(backendError);

  const { useCreateBooking } = await import('./useCreateBooking');
  const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createQueryWrapper() });

  result.current.mutate({
    court_id: 'ct1',
    date: '2026-03-18',
    start_time: '10:00',
    duration_minutes: 90,
    client_phone: '1155550000',
    client_first_name: 'Juan',
    client_last_name: 'Perez',
  });

  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useCreateBooking — onError surfaces the real backend message', () => {
  it('shows the backend error message from error.data instead of the generic fallback', async () => {
    await triggerCreateError(
      await makeConsumedHttpError(400, {
        error: 'El cliente tiene una reserva superpuesta',
      }),
    );

    expect(toast.error).toHaveBeenCalledWith('El cliente tiene una reserva superpuesta');
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.bookings.createError);
  });

  it('falls back to the generic i18n message when the backend body has no error field', async () => {
    await triggerCreateError(await makeConsumedHttpError(500, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.bookings.createError);
  });

  // The server now refuses several distinct slot problems on 409 (no price
  // configured, outside schedule, closed that day, ...) as plain English
  // prose with no error code attached. The client can't map prose to
  // Spanish copy without string-matching a sentence that may be reworded,
  // so it owns the headline and shows the server's sentence verbatim as
  // the toast description instead of guessing which case happened.
  it('shows a client-owned Spanish headline with the server sentence as detail on 409', async () => {
    await triggerCreateError(
      await makeConsumedHttpError(409, {
        error: 'the selected time has no price configured and cannot be booked',
      }),
    );

    expect(toast.error).toHaveBeenCalledWith(ES_AR.bookings.createError, {
      description: 'the selected time has no price configured and cannot be booked',
    });
  });

  it('falls back to the generic slot-occupied detail on 409 when the body has no error field', async () => {
    await triggerCreateError(await makeConsumedHttpError(409, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.bookings.createError, {
      description: ES_AR.bookings.slotOccupied,
    });
  });

  // onError used to read error.response.status directly, which crashes for
  // anything that isn't ky's HTTPError (a dropped connection throws a plain
  // TypeError with no .response) — the mutation would go stuck in a broken
  // state with no toast shown at all instead of a generic error message.
  // That message is now the generic connectivity one, not this mutation's
  // own fallback (ERR-04) — a dropped connection never reached the server,
  // so it shouldn't read as "we couldn't create the booking".
  it('shows the generic connectivity toast instead of crashing when the request fails with a network error', async () => {
    await triggerCreateError(
      new TypeError('Failed to fetch') as unknown as Awaited<ReturnType<typeof makeConsumedHttpError>>,
    );

    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.networkError);
  });

  afterEach(async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.create).mockReset();
  });
});
