import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeConsumedHttpError } from '@/test/factories';
import type { BookingSlotInfo } from '@/features/public-booking';

const t = ES_AR;

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const createBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('@/features/public-booking/api/public-booking.api', () => ({
  publicBookingApi: {
    createBooking: (...args: unknown[]) => createBooking(...args),
  },
}));

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

// Renders whatever `useConfirmBookingSubmit` navigates to `/success` with,
// so a test can assert the confirm flow carried the token forward rather
// than just checking that *some* navigation happened.
function SuccessRouteProbe() {
  const location = useLocation();
  const state = location.state as { token?: string } | null;
  return <div>success page - token:{state?.token ?? 'none'}</div>;
}

// Exposes the query string the page navigated back to, so a test can assert
// on it with URLSearchParams instead of a fragile literal string.
function SlotSelectionRouteProbe() {
  const location = useLocation();
  return <div>slot selection page - query:{location.search}</div>;
}

async function renderPage(state: BookingSlotInfo | null = mockSlotInfo) {
  const Page = (await import('./BookConfirmPage')).default;
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[{ pathname: '/club-norte/book/confirm', state }]}>
        <Routes>
          <Route path="/:slug/book/confirm" element={<Page />} />
          <Route path="/:slug" element={<SlotSelectionRouteProbe />} />
          <Route path="/:slug/book/success" element={<SuccessRouteProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('BookConfirmPage redirect guard', () => {
  it('redirects to the slot selection page when no slotInfo state is present', async () => {
    await renderPage(null);
    await waitFor(() => {
      expect(screen.getByText(/slot selection page/)).toBeInTheDocument();
    });
  });

  it('redirects to the slot selection page when location.state is garbage, not a valid BookingSlotInfo', async () => {
    // `location.state` is `history.state` — it survives a refresh and can
    // carry anything a previous, unrelated navigation left there.
    await renderPage({ some: 'unrelated shape' } as unknown as BookingSlotInfo);
    await waitFor(() => {
      expect(screen.getByText(/slot selection page/)).toBeInTheDocument();
    });
  });

  it('redirects when a required field is present but the wrong type (would otherwise reach pricing as NaN)', async () => {
    const badSlotInfo = { ...mockSlotInfo, price: 'not-a-number' } as unknown as BookingSlotInfo;
    await renderPage(badSlotInfo);
    await waitFor(() => {
      expect(screen.getByText(/slot selection page/)).toBeInTheDocument();
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
  it('renders the booking form with the passed slotInfo', async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });
  });

  it('navigates to the success page carrying the token, not a booking id, when booking succeeds without an mp_init_point', async () => {
    createBooking.mockResolvedValue({
      booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' },
      token: 'tok-abc123',
    });
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmitConfirmForm(user);

    await waitFor(
      () => {
        expect(screen.getByText('success page - token:tok-abc123')).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
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
    const restoreSessionStorage = stubThrowingSessionStorage();
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
    const user = userEvent.setup();
    await renderPage();
    await fillAndSubmit(user);
    await screen.findByRole('alert');

    await user.click(screen.getByRole('button', { name: t.publicBooking.retryPaymentLink }));

    await waitFor(() => {
      expect(screen.getByText('success page - token:tok-retry-1')).toBeInTheDocument();
    });
    expect(createBooking).toHaveBeenCalledTimes(2);
  });
});

describe('BookConfirmPage back link', () => {
  it('returns to the complex page with date, duration, time and sport in the query', async () => {
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    const queryText = await waitFor(() => {
      const el = screen.getByText(/slot selection page - query:/);
      return el.textContent;
    });
    const query = new URLSearchParams(queryText.replace('slot selection page - query:', ''));

    expect(query.get('date')).toBe(mockSlotInfo.date);
    expect(query.get('duration')).toBe(String(mockSlotInfo.durationMinutes));
    expect(query.get('time')).toBe(mockSlotInfo.startTime);
    expect(query.get('sport')).toBe('padel');
  });

  it('leaves the sport key out of the query when the slotInfo has none', async () => {
    const user = userEvent.setup();
    const { sport: _sport, ...withoutSport } = mockSlotInfo;
    await renderPage(withoutSport);
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: t.publicBooking.changeTimeSlot }));

    const queryText = await waitFor(() => {
      const el = screen.getByText(/slot selection page - query:/);
      return el.textContent;
    });
    const query = new URLSearchParams(queryText.replace('slot selection page - query:', ''));

    expect(query.has('sport')).toBe(false);
  });
});
