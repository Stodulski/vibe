import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import {
  HOURS_URL,
  firePagehide,
  fireRestore,
  holdGone,
  inSeconds,
  renderHeld,
  setUpConfirmHold,
} from '../test/confirmHold';
import { readConfirmDraft } from '../lib/handoff';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const reclaimHold = vi.fn<(token: string) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: vi.fn(),
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
    // The server answers the same instant; the page keeps its own deadline
    // (clientHoldDeadline), so a different answer must not move the countdown.
    reclaimHold.mockResolvedValue({ expiresAt: inSeconds(14 * 60) });

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
