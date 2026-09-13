import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { PublicBookingRequest } from '@/shared/types/api.types';
import { usePublicBooking } from './usePublicBooking';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * These go through the real `publicBookingApi` and the real ky client, with
 * MSW answering `POST /book` — the whole point is the problem+json `kind`,
 * which only exists because `ky.ts`'s `beforeError` hook read the body into
 * an `ApiError`. A mocked api module would have to hand-build that object and
 * would then be asserting its own fixture rather than the parse.
 */
const VALID_REQUEST: PublicBookingRequest = {
  complex_id: 'c1',
  court_id: 'ct1',
  date: '2026-03-18',
  start_time: '10:00',
  duration_minutes: 90,
  client_first_name: 'Juan',
  client_last_name: 'Perez',
  client_phone: '1155550000',
  client_email: 'juan@test.com',
};

/** Answers `POST /book` with one 409 of the given problem kind. */
function refuseWith(kind: string, detail: string) {
  server.use(
    http.post('*/book', () =>
      HttpResponse.json(
        { type: `https://vibe.com.ar/problems/${kind}`, title: 'Conflict', status: 409, detail },
        { status: 409 },
      ),
    ),
  );
}

async function submit(onConflict: () => void) {
  const { result } = renderHook(() => usePublicBooking({ onConflict }), { wrapper: createQueryWrapper() });
  result.current.mutate(VALID_REQUEST);
  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('usePublicBooking — 409 by problem kind', () => {
  it('tells a slot-unavailable refusal apart and sends the visitor back to slot selection', async () => {
    refuseWith('slot-unavailable', 'the selected time slot is no longer available, please choose another');
    const onConflict = vi.fn();

    await submit(onConflict);

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.slotConflict);
    expect(onConflict).toHaveBeenCalledOnce();
  });

  // The booking the visitor is trying to make already exists under their own
  // name. Re-picking the same hours would only earn the same refusal, so the
  // flow must not bounce them back to the picker.
  it('tells a duplicate-booking refusal apart and keeps the visitor where they are', async () => {
    refuseWith('duplicate-booking', 'the selected time slot is no longer available, please choose another');
    const onConflict = vi.fn();

    await submit(onConflict);

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.duplicateBooking);
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.publicBooking.slotConflict);
    expect(onConflict).not.toHaveBeenCalled();
  });

  it('falls back to the generic conflict copy for a stale-version 409', async () => {
    refuseWith('stale-version', 'unable to update the record due to an edit conflict, please try again');
    const onConflict = vi.fn();

    await submit(onConflict);

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.bookingConflict);
    expect(onConflict).not.toHaveBeenCalled();
  });

  it('falls back to the generic conflict copy for a 409 of any other kind', async () => {
    refuseWith('conflict', 'the booking is no longer confirmable');
    const onConflict = vi.fn();

    await submit(onConflict);

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.bookingConflict);
    expect(onConflict).not.toHaveBeenCalled();
  });

  // A 409 whose body is not problem+json at all leaves `kind` undefined; it
  // must still land on the generic copy rather than on the slot story.
  it('falls back to the generic conflict copy for a 409 with no problem body', async () => {
    server.use(http.post('*/book', () => new HttpResponse(null, { status: 409 })));
    const onConflict = vi.fn();

    await submit(onConflict);

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.bookingConflict);
    expect(onConflict).not.toHaveBeenCalled();
  });
});
