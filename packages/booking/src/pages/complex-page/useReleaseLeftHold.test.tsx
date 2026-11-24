import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import { createBookingWrapper } from '../../test/booking';
import { createBookingQueryClient } from '../../lib/bookingQueryClient';
import { readConfirmDraft, saveConfirmDraft, type ConfirmDraft } from '../../lib/handoff';
import { useReleaseLeftHold } from './useReleaseLeftHold';

const SLUG = 'club-norte';

const DRAFT: ConfirmDraft = {
  complexId: 'c1',
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1_000_000,
  depositPercentage: 30,
  cancellationHours: 24,
  sport: 'padel',
  holdToken: 'hold-abc',
  holdExpiresAt: new Date(Date.now() + 9 * 60_000).toISOString(),
};

/** Records the token of every immediate release. */
function captureReleases() {
  const released: string[] = [];
  server.use(
    http.post('*/book/holds/:token/release', ({ params }) => {
      released.push(String(params.token));
      return new HttpResponse(null, { status: 204 });
    }),
  );
  return released;
}

function renderReleaseHook() {
  const queryClient = createBookingQueryClient();
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
  renderHook(() => {
    useReleaseLeftHold(SLUG);
  }, { wrapper: createBookingWrapper({ queryClient }) });
  return invalidate;
}

function fireRestore(persisted = true) {
  const event = new Event('pageshow') as PageTransitionEvent;
  Object.defineProperty(event, 'persisted', { value: persisted });
  window.dispatchEvent(event);
}

afterEach(() => {
  window.sessionStorage.clear();
});

describe('useReleaseLeftHold', () => {
  it('gives back at once the hold of a confirm page the visitor came back from, and refreshes the hours', async () => {
    const released = captureReleases();
    saveConfirmDraft(SLUG, DRAFT);

    const invalidate = renderReleaseHook();

    expect(readConfirmDraft(SLUG)).toBeNull();
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ['availability', SLUG] });
    });
    expect(released).toEqual(['hold-abc']);
  });

  it('releases nothing when no hold is waiting', async () => {
    const released = captureReleases();

    const invalidate = renderReleaseHook();

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(released).toEqual([]);
    expect(invalidate).not.toHaveBeenCalled();
  });

  it('keeps the hold this page just took for the confirm page it is going to', async () => {
    const released = captureReleases();
    renderReleaseHook();

    // The court pick: the hold is saved, then the browser leaves for confirm.
    saveConfirmDraft(SLUG, DRAFT);

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(released).toEqual([]);
    expect(readConfirmDraft(SLUG)?.holdToken).toBe('hold-abc');
  });

  it('gives the hold back when the browser shows this page again from its back/forward cache', async () => {
    const released = captureReleases();
    renderReleaseHook();
    saveConfirmDraft(SLUG, DRAFT);

    fireRestore();

    await waitFor(() => {
      expect(released).toEqual(['hold-abc']);
    });
    expect(readConfirmDraft(SLUG)).toBeNull();
  });

  it('ignores a pageshow that is not a restore', async () => {
    const released = captureReleases();
    renderReleaseHook();
    saveConfirmDraft(SLUG, DRAFT);

    fireRestore(false);

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(released).toEqual([]);
  });
});
