import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderBooking, setBookingUrl } from '../test/booking';
import { BOOKING_MESSAGES } from '../messages';
import { makeConsumedHttpError } from '../test/factories';
import type { BookingConfig } from '../config';
import { confirmDraftKey, readConfirmDraft, hasHoldExpiredNotice } from '../lib/handoff';

const t = BOOKING_MESSAGES;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: (...args: unknown[]) => createBooking(...args),
  };
  return { publicBookingApi, createPublicBookingApi: () => publicBookingApi };
});

// Records whether the confirm step still mounts a Turnstile widget.
vi.mock('@marsidev/react-turnstile', () => ({
  Turnstile: () => <div data-testid="turnstile-widget" />,
}));

const SLOT = {
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
};

const CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
  turnstileSiteKey: 'test-site-key',
};

const RELEASE_URL = 'http://localhost/api/v1/book/holds/hold-abc/release';
const HOURS_URL = '/c/club-norte?date=2026-03-20&duration=90&sport=padel#horarios';

function inSeconds(seconds: number): string {
  return new Date(Date.now() + seconds * 1000).toISOString();
}

let replaceSpy: MockInstance<Location['replace']>;
let beacon: ReturnType<typeof vi.fn<(url: string | URL, data?: BodyInit | null) => boolean>>;

beforeEach(() => {
  vi.clearAllMocks();
  replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
  beacon = vi.fn<(url: string | URL, data?: BodyInit | null) => boolean>(() => true);
  Object.defineProperty(navigator, 'sendBeacon', { configurable: true, writable: true, value: beacon });
});

afterEach(() => {
  replaceSpy.mockRestore();
  setBookingUrl('/');
  window.sessionStorage.clear();
});

async function renderPage(draft: unknown) {
  const Page = (await import('./BookConfirmPage')).default;
  setBookingUrl('/c/club-norte/book/confirm');
  window.sessionStorage.setItem(confirmDraftKey('club-norte'), JSON.stringify(draft));
  return renderBooking(<Page slug="club-norte" />, { config: CONFIG });
}

async function renderHeld(expiresInSeconds = 15 * 60) {
  await renderPage({ ...SLOT, holdToken: 'hold-abc', holdExpiresAt: inSeconds(expiresInSeconds) });
  await screen.findByText('Cancha 1');
}

async function fillAndPay(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/nombre/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1122334455');
  await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
  await user.click(screen.getByRole('button', { name: /pagar|reservar/i }));
}

function firePagehide() {
  window.dispatchEvent(new Event('pagehide'));
}

describe('BookConfirmPage hold guard', () => {
  it('redirects to the complex page when the draft carries no hold', async () => {
    await renderPage(SLOT);

    await waitFor(() => {
      expect(replaceSpy).toHaveBeenCalledWith('/c/club-norte');
    });
  });
});

describe('BookConfirmPage countdown', () => {
  it('shows the time left on the hold, from the server expiry', async () => {
    await renderHeld(10 * 60 + 5);

    // Only the time is on screen; the sentence is the timer's accessible name.
    const timer = screen.getByRole('timer', { name: t.publicBooking.holdCountdownLabel });
    expect(timer).toHaveTextContent(/^10:05$/);
  });

  it('tells the visitor the hold expired and sends them back to the hours when the time is up', async () => {
    await renderPage({ ...SLOT, holdToken: 'hold-abc', holdExpiresAt: inSeconds(-1) });

    await waitFor(() => {
      expect(replaceSpy).toHaveBeenCalledWith(HOURS_URL);
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
      expect(replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    });
    expect(hasHoldExpiredNotice('club-norte')).toBe(true);
  });
});

describe('BookConfirmPage hold release', () => {
  it('"Cambiar horario" releases the hold and returns to the hours', async () => {
    const user = userEvent.setup();
    await renderHeld();

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    expect(beacon).toHaveBeenCalledWith(RELEASE_URL);
    expect(replaceSpy).toHaveBeenCalledWith(HOURS_URL);
    // The page leaving afterwards does not release it a second time.
    firePagehide();
    expect(beacon).toHaveBeenCalledTimes(1);
  });

  it('releases the hold with a beacon when the page goes away (back, close)', async () => {
    await renderHeld();

    firePagehide();

    expect(beacon).toHaveBeenCalledWith(RELEASE_URL);
    // A reload of this page must not show a hold that no longer exists.
    expect(readConfirmDraft('club-norte')).toBeNull();
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

      expect(beacon).not.toHaveBeenCalled();
    } finally {
      Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
    }
  });
});
