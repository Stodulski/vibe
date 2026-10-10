import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderBooking, setBookingUrl } from '@/test/booking';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeConsumedHttpError } from '@/test/factories';
import type { BookingSlotInfo } from '@/features/public-booking';
import { confirmDraftKey, readBookingResult } from '../lib/handoff';

const t = ES_AR;

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('@/features/public-booking/api/public-booking.api', () => {
  const publicBookingApi = {
    createBooking: (...args: unknown[]) => createBooking(...args),
  };
  // The page reads this through BookingRoot's createPublicBookingApi.
  return { publicBookingApi, createPublicBookingApi: () => publicBookingApi };
});

const mockSlotInfo: BookingSlotInfo = {
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

let replaceSpy: MockInstance<Location['replace']> | undefined;

/** Stops the page's redirects from leaving the test document, and records them. */
function spyOnReplace() {
  replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
  return replaceSpy;
}

/** The URL the page last sent the browser to, as a URL, so a test can read its query. */
function lastRedirect(spy: { mock: { calls: unknown[][] } }): URL {
  return new URL(String(spy.mock.calls.at(-1)?.[0]), 'http://localhost');
}

/**
 * Renders the confirm page. The complex page leaves the chosen slot as a draft for
 * this slug, which is what the page reads on mount. A test can park another value
 * there, raw, to check that the schema rejects it.
 */
async function renderPage(draft: unknown = mockSlotInfo) {
  const Page = (await import('./BookConfirmPage')).default;
  setBookingUrl('/c/club-norte/book/confirm');
  if (draft !== null) {
    window.sessionStorage.setItem(confirmDraftKey('club-norte'), JSON.stringify(draft));
  }
  return renderBooking(<Page slug="club-norte" />);
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  replaceSpy?.mockRestore();
  replaceSpy = undefined;
  setBookingUrl('/');
  window.sessionStorage.clear();
});

describe('BookConfirmPage redirect guard', () => {
  it('redirects to the complex page when no draft is saved', async () => {
    const replace = spyOnReplace();
    await renderPage(null);
    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith('/c/club-norte');
    });
  });

  it('redirects to the complex page when the stored draft is not a valid BookingSlotInfo', async () => {
    // sessionStorage is editable and survives a refresh, so it can hold anything an
    // earlier visit left there.
    const replace = spyOnReplace();
    await renderPage({ some: 'unrelated shape' });
    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith('/c/club-norte');
    });
  });

  it('redirects when a required field is present but the wrong type (would otherwise reach pricing as NaN)', async () => {
    const replace = spyOnReplace();
    const badSlotInfo = { ...mockSlotInfo, price: 'not-a-number' };
    await renderPage(badSlotInfo);
    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith('/c/club-norte');
    });
  });
});

async function fillAndSubmitConfirmForm(user: ReturnType<typeof userEvent.setup>) {
  await waitFor(() => {
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
  });
  await user.type(screen.getByLabelText(/nombre/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1122334455');
  await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
  await user.click(screen.getByRole('button', { name: /pagar|reservar/i }));
}

/** Swaps `window.location` for a plain writable stub, restored by the returned callback. */
function stubWindowLocation() {
  const originalLocation = window.location;
  Object.defineProperty(window, 'location', { configurable: true, value: { href: '' } });
  return () => {
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
  };
}

/**
 * Replaces `window.sessionStorage` wholesale with a stub whose `setItem`
 * always throws — happy-dom's real `sessionStorage` is Proxy-backed, so
 * `vi.spyOn` on it (or on `Storage.prototype`) silently fails to intercept
 * calls, which is why this swaps the whole object instead. Restored by the
 * returned callback.
 */
function stubThrowingSessionStorage() {
  const original = window.sessionStorage;
  Object.defineProperty(window, 'sessionStorage', {
    configurable: true,
    value: {
      getItem: () => null,
      setItem: () => {
        throw new DOMException('QuotaExceededError');
      },
      removeItem: () => {
        /* noop stub */
      },
      clear: () => {
        /* noop stub */
      },
      key: () => null,
      length: 0,
    },
  });
  return () => {
    Object.defineProperty(window, 'sessionStorage', { configurable: true, value: original });
  };
}

describe('BookConfirmPage happy path', () => {
  it('renders the booking form with the draft slot', async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });
  });

  it('saves the token as the booking result and moves to the success page when there is no mp_init_point', async () => {
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-abc123',
    });
    const replace = spyOnReplace();
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmitConfirmForm(user);

    await waitFor(
      () => {
        expect(replace).toHaveBeenCalledWith('/c/club-norte/book/success');
      },
      { timeout: 3000 },
    );
    expect(readBookingResult('club-norte')?.token).toBe('tok-abc123');
  });

  it('redirects to Mercado Pago when mp_init_point is present', async () => {
    const restoreLocation = stubWindowLocation();
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-abc123',
      mp_init_point: 'https://mp.com/checkout/b1',
    });
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmitConfirmForm(user);

    await waitFor(() => {
      expect(window.location.href).toBe('https://mp.com/checkout/b1');
    });
    restoreLocation();
  });

  // The booking is already created server-side by this point — losing the
  // sessionStorage cache in Safari private browsing (or a full quota) must
  // never leave the person stuck here with a slot the server thinks is taken.
  it('still redirects to Mercado Pago when caching bookingInfo to sessionStorage throws', async () => {
    const restoreLocation = stubWindowLocation();
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-abc123',
      mp_init_point: 'https://mp.com/checkout/b1',
    });
    const user = userEvent.setup();
    // Rendered first: the draft is written before the storage stub goes in.
    await renderPage();
    const restoreSessionStorage = stubThrowingSessionStorage();
    await fillAndSubmitConfirmForm(user);

    await waitFor(() => {
      expect(window.location.href).toBe('https://mp.com/checkout/b1');
    });
    restoreSessionStorage();
    restoreLocation();
  });
});

describe('BookConfirmPage payment link error (503)', () => {
  async function fillAndSubmit(user: ReturnType<typeof userEvent.setup>) {
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });
    await user.type(screen.getByLabelText(/nombre/i), 'Juan');
    await user.type(screen.getByLabelText(/apellido/i), 'Perez');
    await user.type(screen.getByLabelText(/tel.fono/i), '1122334455');
    await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
    await user.click(screen.getByRole('button', { name: /pagar|reservar/i }));
  }

  it('shows a persistent inline error, not just a toast, when the server cannot create the payment link', async () => {
    createBooking.mockRejectedValueOnce(await makeConsumedHttpError(503, {}));
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmit(user);

    expect(await screen.findByRole('alert')).toHaveTextContent(t.publicBooking.paymentLinkError);
    // Still there after the microtask queue settles — not a toast that
    // dismisses itself.
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
    expect(screen.getByRole('alert')).toHaveTextContent(t.publicBooking.paymentLinkError);
  });

  it('resubmits the same data when "Reintentar" is clicked, without asking the person to type it again', async () => {
    createBooking.mockRejectedValueOnce(await makeConsumedHttpError(503, {}));
    createBooking.mockResolvedValueOnce({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-retry-1',
    });
    const replace = spyOnReplace();
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmit(user);
    await screen.findByRole('alert');

    await user.click(screen.getByRole('button', { name: t.publicBooking.retryPaymentLink }));

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith('/c/club-norte/book/success');
    });
    expect(readBookingResult('club-norte')?.token).toBe('tok-retry-1');
    expect(createBooking).toHaveBeenCalledTimes(2);
  });
});

describe('BookConfirmPage back link', () => {
  it('returns to the complex page with date, duration, time and sport in the query', async () => {
    const replace = spyOnReplace();
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    await waitFor(() => {
      expect(replace).toHaveBeenCalled();
    });
    const query = lastRedirect(replace).searchParams;

    expect(lastRedirect(replace).pathname).toBe('/c/club-norte');
    expect(query.get('date')).toBe(mockSlotInfo.date);
    expect(query.get('duration')).toBe(String(mockSlotInfo.durationMinutes));
    expect(query.get('time')).toBe(mockSlotInfo.startTime);
    expect(query.get('sport')).toBe('padel');
  });

  it('leaves the sport key out of the query when the slotInfo has none', async () => {
    const replace = spyOnReplace();
    const user = userEvent.setup();
    const { sport: _sport, ...withoutSport } = mockSlotInfo;
    await renderPage(withoutSport);
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    await waitFor(() => {
      expect(replace).toHaveBeenCalled();
    });
    expect(lastRedirect(replace).searchParams.has('sport')).toBe(false);
  });
});
