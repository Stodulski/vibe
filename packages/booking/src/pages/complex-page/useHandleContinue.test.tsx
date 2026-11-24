import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '../../test/msw/server';
import { createBookingWrapper, setBookingUrl } from '../../test/booking';
import type { PublicComplex } from '../../api/types';
import type { SelectedSlot } from '../../components/CourtSelector';
import { useHandleContinue } from './useHandleContinue';
import type { HoldChallenge } from './useHoldChallenge';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

const complex = {
  id: 'c1',
  name: 'Club Test',
  phone: '+541100000000',
  deposit_percentage: 30,
  cancellation_hours: 24,
} as PublicComplex;

const selection: SelectedSlot = {
  courtId: 'ct1',
  courtName: 'Cancha 1',
  sport: 'padel',
  courtType: 'indoor',
  hasLighting: false,
  slot: { start_time: '10:00', end_time: '11:00', start_min: 600, duration_minutes: 60, price: 5000, available: true },
  durationMinutes: 60,
  totalPrice: 5000,
  endTime: '11:00',
};

const noChallenge: HoldChallenge = { required: false, token: undefined, reset: () => undefined };

/** Counts hold requests and answers each with `respond`. */
function countHolds(respond: () => Response) {
  const count = { holds: 0 };
  server.use(
    http.post('*/book/holds', () => {
      count.holds += 1;
      return respond();
    }),
  );
  return count;
}

function renderContinue(challenge: HoldChallenge = noChallenge) {
  return renderHook(() => useHandleContinue('test-club', complex, true, selection, '2030-01-01', challenge), {
    wrapper: createBookingWrapper(),
  });
}

let assignSpy: MockInstance<Location['assign']> | undefined;

beforeEach(() => {
  vi.clearAllMocks();
  setBookingUrl('/c/test-club');
  assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);
});

afterEach(() => {
  assignSpy?.mockRestore();
  setBookingUrl('/');
});

describe('useHandleContinue: one intent, one hold', () => {
  it('sends no second hold when the court is tapped again while the page leaves for confirm', async () => {
    const count = countHolds(() =>
      HttpResponse.json({ hold: { token: 'hold-new', expires_at: '2030-01-01T12:15:00Z' } }, { status: 201 }),
    );
    const { result } = renderContinue();

    act(() => {
      result.current(selection);
    });
    await waitFor(() => {
      expect(assignSpy).toHaveBeenCalledTimes(1);
    });
    // "Reservar" tapped while the confirm page is still loading.
    act(() => {
      result.current(selection);
    });
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(count.holds).toBe(1);
  });

  it('shows one refusal at a time: a repeated refusal replaces the toast instead of stacking', async () => {
    const count = countHolds(() => HttpResponse.json({ type: 'about:blank', status: 500 }, { status: 500 }));
    const { result } = renderContinue();

    for (let tap = 1; tap <= 2; tap++) {
      act(() => {
        result.current(selection);
      });
      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledTimes(tap);
      });
    }

    expect(count.holds).toBe(2);
    const [first, second] = vi.mocked(toast.error).mock.calls;
    expect(first?.[1]?.id).toBeDefined();
    expect(second?.[1]?.id).toBe(first?.[1]?.id);
  });

  it('asks once to wait for the check when the court is tapped twice before it is solved', () => {
    const { result } = renderContinue({ required: true, token: undefined, reset: () => undefined });

    act(() => {
      result.current(selection);
      result.current(selection);
    });

    const ids = vi.mocked(toast.info).mock.calls.map((call) => call[1]?.id);
    expect(new Set(ids).size).toBe(1);
    expect(ids[0]).toBeDefined();
  });
});

const COOLDOWN = 'Vas a poder elegirlo de nuevo desde las 19:40.';

function refuseWithCooldown() {
  return countHolds(() =>
    HttpResponse.json(
      { type: 'https://vibe.com.ar/problems/hold-cooldown', title: '', status: 409, detail: COOLDOWN },
      { status: 409 },
    ),
  );
}

/** Renders the hook behind a challenge whose token the test swaps, as the page's widget does. */
function renderWithToken(token: string | undefined) {
  const reset = () => undefined;
  const challenge = (value: string | undefined): HoldChallenge => ({ required: true, token: value, reset });
  const view = renderHook(
    ({ current }: { current: HoldChallenge }) =>
      useHandleContinue('test-club', complex, true, selection, '2030-01-01', current),
    { wrapper: createBookingWrapper(), initialProps: { current: challenge(token) } },
  );
  return {
    tap: () => {
      act(() => {
        view.result.current(selection);
      });
    },
    setToken: (value: string | undefined) => {
      view.rerender({ current: challenge(value) });
    },
  };
}

describe('useHandleContinue: a refusal stays the answer for a quick second tap', () => {
  it('shows the same refusal again and sends no second hold', async () => {
    const count = refuseWithCooldown();
    const page = renderWithToken('tok-1');

    page.tap();
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(COOLDOWN, expect.anything());
    });
    // The refused hold spent the token; "Reservar" is tapped before and after the fresh one arrives.
    page.setToken(undefined);
    page.tap();
    page.setToken('tok-2');
    page.tap();
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(count.holds).toBe(1);
    expect(toast.info).not.toHaveBeenCalled();
    const refusals = vi.mocked(toast.error).mock.calls;
    expect(refusals.map((call) => call[0])).toEqual([COOLDOWN, COOLDOWN, COOLDOWN]);
    expect(new Set(refusals.map((call) => call[1]?.id)).size).toBe(1);
  });

  it('sends a deliberate retry made a little later', async () => {
    const count = refuseWithCooldown();
    const page = renderWithToken('tok-1');
    page.tap();
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledTimes(1);
    });
    page.setToken('tok-2');

    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 10_000);
    try {
      page.tap();
      await waitFor(() => {
        expect(count.holds).toBe(2);
      });
    } finally {
      clock.mockRestore();
    }
  });
});
