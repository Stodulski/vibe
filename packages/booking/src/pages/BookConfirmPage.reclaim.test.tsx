import { afterEach, describe, it, expect, vi } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError } from '../test/factories';
import {
  HOURS_URL,
  fillAndPay,
  firePagehide,
  fireRestore,
  holdGone,
  renderHeld,
  setUpConfirmHold,
} from '../test/confirmHold';
import { readConfirmDraft } from '../lib/handoff';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const reclaimHold = vi.fn<(token: string) => Promise<void>>();
const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: (...args: unknown[]) => createBooking(...args),
    reclaimHold: (token: string) => reclaimHold(token),
  };
  return { publicBookingApi, createPublicBookingApi: () => publicBookingApi };
});

vi.mock('@marsidev/react-turnstile', () => ({
  Turnstile: () => <div data-testid="turnstile-widget" />,
}));

const env = setUpConfirmHold(reclaimHold);

// Leaving the page lets the hold go a minute later (owner decision
// 2026-11-24): a page shown again within that minute takes it back.

describe('BookConfirmPage loaded again (a reload, a discarded tab)', () => {
  it('takes the hold back and keeps the countdown where it was', async () => {
    await renderHeld(6 * 60);

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledWith('hold-abc');
    });
    expect(screen.getByRole('timer')).toHaveTextContent(/^0[56]:\d\d$/);
    expect(env.replaceSpy).not.toHaveBeenCalled();
    expect(readConfirmDraft('club-norte')?.holdToken).toBe('hold-abc');
  });

  it('sends the visitor to the hours when the hold is gone', async () => {
    reclaimHold.mockReturnValue(holdGone());

    await renderHeld();

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(readConfirmDraft('club-norte')).toBeNull();
    // Going away now must not touch the hold it no longer has.
    firePagehide();
    expect(env.beacon).not.toHaveBeenCalled();
  });

  it('stays on the page when the hold could not be asked about', async () => {
    reclaimHold.mockRejectedValue(new TypeError('Failed to fetch'));

    await renderHeld();

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledWith('hold-abc');
    });
    expect(env.replaceSpy).not.toHaveBeenCalled();
    expect(readConfirmDraft('club-norte')?.holdToken).toBe('hold-abc');
  });
});

describe('BookConfirmPage restored from the back/forward cache within the grace', () => {
  it('sends the visitor to the hours when the complex page gave the hold back meanwhile', async () => {
    await renderHeld();
    firePagehide();
    // Back on the complex page, which released the hold and cleared the draft.
    window.sessionStorage.clear();
    reclaimHold.mockClear();

    fireRestore();

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(reclaimHold).not.toHaveBeenCalled();
  });

  it('takes the hold back when the visitor returns within the grace', async () => {
    await renderHeld(5 * 60);
    firePagehide();

    fireRestore();

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(2);
    });
    expect(reclaimHold).toHaveBeenLastCalledWith('hold-abc');
    expect(env.replaceSpy).not.toHaveBeenCalled();
    expect(screen.getByRole('timer')).toHaveTextContent(/^0[45]:\d\d$/);
  });
});

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state });
  document.dispatchEvent(new Event('visibilitychange'));
}

describe('BookConfirmPage asserting its hold again', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('reclaims the hold again a few seconds after loading, so a late leave beacon cannot release it', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await renderHeld();
    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(1);
    });

    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(2);
    });
    expect(reclaimHold).toHaveBeenLastCalledWith('hold-abc');
    expect(env.replaceSpy).not.toHaveBeenCalled();
  });

  it('sends the visitor to the hours when the hold is gone by the second reclaim', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await renderHeld();
    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(1);
    });
    reclaimHold.mockReturnValueOnce(holdGone());

    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(readConfirmDraft('club-norte')).toBeNull();
  });

});

describe('BookConfirmPage reclaiming at other moments', () => {
  it('reclaims the hold when the visitor returns to the tab', async () => {
    await renderHeld();
    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(1);
    });

    setVisibility('hidden');
    setVisibility('visible');

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledTimes(2);
    });
  });

  it('stays on the page when the reclaim route is not there (404)', async () => {
    reclaimHold.mockImplementation(async () => {
      throw await makeConsumedHttpError(404, { type: 'about:blank', title: 'Not Found', status: 404 });
    });

    await renderHeld();

    await waitFor(() => {
      expect(reclaimHold).toHaveBeenCalledWith('hold-abc');
    });
    expect(env.replaceSpy).not.toHaveBeenCalled();
    expect(readConfirmDraft('club-norte')?.holdToken).toBe('hold-abc');
  });

  it('never leaves a booking being submitted for a late reclaim answer', async () => {
    let answerReclaim: (error: unknown) => void = () => undefined;
    reclaimHold.mockImplementationOnce(
      () =>
        new Promise<void>((_, reject) => {
          answerReclaim = reject;
        }),
    );
    createBooking.mockImplementation(() => new Promise(() => undefined));
    const user = userEvent.setup();
    await renderHeld();

    await fillAndPay(user);
    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    answerReclaim(await holdGone().catch((error: unknown) => error));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(env.replaceSpy).not.toHaveBeenCalled();
  });
});
