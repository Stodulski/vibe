import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BOOKING_MESSAGES } from '../messages';
import { makeConsumedHttpError } from '../test/factories';
import {
  HOURS_URL,
  LEAVE_URL,
  RELEASE_URL,
  SLOT,
  fillAndPay,
  firePagehide,
  fireRestore,
  holdGone,
  inSeconds,
  renderHeld,
  renderPage,
  setUpConfirmHold,
} from '../test/confirmHold';
import { confirmDraftKey, readConfirmDraft, hasHoldExpiredNotice } from '../lib/handoff';

const t = BOOKING_MESSAGES;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const reclaimHold = vi.fn<(token: string) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: (...args: unknown[]) => createBooking(...args),
    reclaimHold: (token: string) => reclaimHold(token),
  };
  return { publicBookingApi, createPublicBookingApi: () => publicBookingApi };
});

// Records whether the confirm step still mounts a Turnstile widget.
vi.mock('@marsidev/react-turnstile', () => ({
  Turnstile: () => <div data-testid="turnstile-widget" />,
}));

const env = setUpConfirmHold(reclaimHold);

describe('BookConfirmPage hold guard', () => {
  it('redirects to the complex page when the draft carries no hold', async () => {
    await renderPage(SLOT);

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith('/c/club-norte');
    });
  });
});

describe('BookConfirmPage countdown', () => {
  it('shows the time left on the hold, from the server expiry', async () => {
    await renderHeld(9 * 60 + 5);

    // Only the time is on screen; the sentence is the timer's accessible name.
    const timer = screen.getByRole('timer', { name: t.publicBooking.holdCountdownLabel });
    expect(timer).toHaveTextContent(/^09:0[45]$/);
  });

  it('tells the visitor the hold expired and sends them back to the hours when the time is up', async () => {
    await renderPage({ ...SLOT, holdToken: 'hold-abc', holdExpiresAt: inSeconds(-1) });

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(hasHoldExpiredNotice('club-norte')).toBe(true);
    expect(readConfirmDraft('club-norte')).toBeNull();
  });
});

describe('BookConfirmPage booking request', () => {
  it('renders no Turnstile widget, even with a site key', async () => {
    await renderHeld();

    expect(screen.queryByTestId('turnstile-widget')).not.toBeInTheDocument();
  });

  it('sends the hold token and no turnstile_token', async () => {
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-1',
    });
    const user = userEvent.setup();
    await renderHeld();

    await fillAndPay(user);

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    const sent = createBooking.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(sent).toMatchObject({ hold_token: 'hold-abc', court_id: 'ct1', start_time: '10:00' });
    expect(sent).not.toHaveProperty('turnstile_token');
  });

  it('treats a 409 hold-expired answer as an expired hold', async () => {
    createBooking.mockRejectedValueOnce(
      await makeConsumedHttpError(409, {
        type: 'https://vibe.com.ar/problems/hold-expired',
        title: 'Hold expired',
        status: 409,
      }),
    );
    const user = userEvent.setup();
    await renderHeld();

    await fillAndPay(user);

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(hasHoldExpiredNotice('club-norte')).toBe(true);
  });
});

describe('BookConfirmPage hold release', () => {
  it('"Cambiar horario" releases the hold and returns to the hours', async () => {
    const user = userEvent.setup();
    await renderHeld();

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    // Given back on purpose: released at once, not after the grace.
    expect(env.beacon).toHaveBeenCalledWith(RELEASE_URL);
    expect(env.beacon).not.toHaveBeenCalledWith(LEAVE_URL);
    expect(readConfirmDraft('club-norte')).toBeNull();
    expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    // The page leaving afterwards does not release it a second time.
    firePagehide();
    expect(env.beacon).toHaveBeenCalledTimes(1);
  });

  it('asks for the release a minute later when the page goes away, and keeps the draft for a reload', async () => {
    await renderHeld();

    firePagehide();

    expect(env.beacon).toHaveBeenCalledTimes(1);
    expect(env.beacon).toHaveBeenCalledWith(LEAVE_URL);
    expect(readConfirmDraft('club-norte')?.holdToken).toBe('hold-abc');
  });

  it('keeps the hold when the page leaves for MercadoPago', async () => {
    const originalLocation = window.location;
    Object.defineProperty(window, 'location', { configurable: true, value: { href: '' } });
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-1',
      mp_init_point: 'https://mp.com/checkout/b1',
    });
    const user = userEvent.setup();
    try {
      await renderHeld();
      await fillAndPay(user);
      await waitFor(() => {
        expect(window.location.href).toBe('https://mp.com/checkout/b1');
      });

      firePagehide();

      expect(env.beacon).not.toHaveBeenCalled();
    } finally {
      Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
    }
  });
});

describe('BookConfirmPage restored from the back/forward cache', () => {
  it('shows the countdown of the hold now in the draft, not the one it was opened with', async () => {
    await renderHeld(5 * 60);
    expect(screen.getByRole('timer')).toHaveTextContent(/^0[45]:\d\d$/);
    // The visitor went back, the page released its hold, and a new slot was held.
    firePagehide();
    window.sessionStorage.setItem(
      confirmDraftKey('club-norte'),
      JSON.stringify({ ...SLOT, startTime: '18:00', holdToken: 'hold-new', holdExpiresAt: inSeconds(9 * 60 + 30) }),
    );

    fireRestore();

    await waitFor(() => {
      expect(screen.getByRole('timer')).toHaveTextContent(/^09:(29|30)$/);
    });
    expect(env.replaceSpy).not.toHaveBeenCalled();
    // Leaving again gives back the new hold, never the released one.
    env.beacon.mockClear();
    firePagehide();
    expect(env.beacon).toHaveBeenCalledTimes(1);
    expect(env.beacon).toHaveBeenCalledWith('http://localhost/api/v1/book/holds/hold-new/leave');
  });

  it('books with the hold now in the draft', async () => {
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-1',
    });
    const user = userEvent.setup();
    await renderHeld();
    firePagehide();
    window.sessionStorage.setItem(
      confirmDraftKey('club-norte'),
      JSON.stringify({ ...SLOT, holdToken: 'hold-new', holdExpiresAt: inSeconds(9 * 60) }),
    );
    fireRestore();
    await waitFor(() => {
      expect(screen.getByRole('timer')).toHaveTextContent(/^0(8:59|9:00)$/);
    });

    await fillAndPay(user);

    await waitFor(() => {
      expect(createBooking).toHaveBeenCalledTimes(1);
    });
    expect(createBooking.mock.calls[0]?.[0]).toMatchObject({ hold_token: 'hold-new' });
  });

  it('sends the visitor to the hours when the hold it showed was released and no other is held', async () => {
    await renderHeld();
    firePagehide();
    reclaimHold.mockReturnValueOnce(holdGone());

    fireRestore();

    await waitFor(() => {
      expect(env.replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(readConfirmDraft('club-norte')).toBeNull();
  });

});

describe('BookConfirmPage back from MercadoPago', () => {
  const originalLocation = window.location;
  let location: { href: string; replace: ReturnType<typeof vi.fn>; assign: ReturnType<typeof vi.fn> };

  beforeEach(() => {
    location = { href: '', replace: vi.fn(), assign: vi.fn() };
    Object.defineProperty(window, 'location', { configurable: true, value: location });
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'link-1',
      mp_init_point: 'https://mp.com/checkout/b1',
    });
  });

  afterEach(() => {
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
  });

  async function payOnMercadoPago() {
    const user = userEvent.setup();
    await renderHeld();
    await fillAndPay(user);
    await waitFor(() => {
      expect(location.href).toBe('https://mp.com/checkout/b1');
    });
    expect(screen.getByText(t.publicBooking.redirectingToMP)).toBeInTheDocument();
  }

  it('shows the booking outcome when the page is restored from the back/forward cache', async () => {
    await payOnMercadoPago();

    fireRestore();

    await waitFor(() => {
      expect(location.replace).toHaveBeenCalledWith('/c/club-norte/book/success?token=link-1');
    });
    // The page did not mistake the converted hold for a released one.
    expect(location.replace).toHaveBeenCalledTimes(1);
  });

  it('shows the booking outcome when the visitor returns from the MercadoPago app to this tab', async () => {
    await payOnMercadoPago();

    setVisibility('hidden');
    expect(location.replace).not.toHaveBeenCalled();
    setVisibility('visible');

    await waitFor(() => {
      expect(location.replace).toHaveBeenCalledWith('/c/club-norte/book/success?token=link-1');
    });
  });
});

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state });
  document.dispatchEvent(new Event('visibilitychange'));
}
