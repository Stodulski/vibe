import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const getCancelInfo = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const cancelBooking = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('@/features/public-booking/api/public-booking.api', () => ({
  publicBookingApi: {
    getCancelInfo: (...args: unknown[]) => getCancelInfo(...args),
    cancelBooking: (...args: unknown[]) => cancelBooking(...args),
  },
}));

const mockCancelInfo = {
  booking: {
    status: 'confirmed',
    date: '2026-03-20',
    start_time: '10:00',
    court_name: 'Cancha 1',
    complex_name: 'Club Norte',
  },
  can_cancel: true,
  can_refund: true,
  refund_method: 'mercadopago',
  cancellation_hours: 24,
};

async function renderPage(token = 't1') {
  const Page = (await import('./BookCancelPage')).default;
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/club-norte/book/cancel?token=${token}`]}>
        <Routes>
          <Route path="/:slug/book/cancel" element={<Page />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('BookCancelPage invalid link', () => {
  it('shows the invalid-link state when no token is present', async () => {
    await renderPage('');
    await waitFor(() => {
      expect(screen.getByText(ES_AR.publicBooking.invalidCancelLink)).toBeInTheDocument();
    });
  });
});

describe('BookCancelPage cancel flow', () => {
  it('renders the booking info and refund-eligible confirm button', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /confirmar cancelaci.n/i })).toBeInTheDocument();
  });

  it('fetches cancel-info with the token from the URL', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    await renderPage('tok-xyz');
    await waitFor(() => {
      expect(getCancelInfo).toHaveBeenCalledWith('tok-xyz', expect.anything());
    });
  });

  it('shows the refund-expired notice when can_refund is false', async () => {
    getCancelInfo.mockResolvedValue({ ...mockCancelInfo, can_refund: false });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: ES_AR.publicBooking.cancelNoRefund })).toBeInTheDocument();
    });
  });

  it('shows an already-cancelled state when can_cancel is false', async () => {
    getCancelInfo.mockResolvedValue({
      ...mockCancelInfo,
      can_cancel: false,
      booking: { ...mockCancelInfo.booking, status: 'cancelled' },
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(/esta reserva ya fue cancelada/i)).toBeInTheDocument();
    });
  });

  it('confirms and shows the success state after cancelling, sending the token in the request body (no refund)', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    cancelBooking.mockResolvedValue({
      refunded: false,
      refund: {
        status: 'not_eligible',
        message: 'La cancelación quedó fuera del plazo de reembolso, así que la seña no se devuelve.',
      },
    });
    const user = userEvent.setup();
    await renderPage('tok-xyz');
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.confirmCancelYes }));

    await waitFor(() => {
      expect(cancelBooking).toHaveBeenCalledWith({ token: 'tok-xyz' });
    });
  });
});

describe('BookCancelPage cancel error handling', () => {
  it('onError surfaces the real backend message from error.data instead of the generic fallback', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    cancelBooking.mockRejectedValue(await makeConsumedHttpError(400, { error: 'El plazo de cancelacion ya vencio' }));
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.confirmCancelYes }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('El plazo de cancelacion ya vencio');
    });
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.publicBooking.cancelBookingError);
  });

  it('onError falls back to the generic i18n message when the backend body has no error field', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    cancelBooking.mockRejectedValue(await makeConsumedHttpError(500, {}));
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.confirmCancelYes }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.cancelBookingError);
    });
  });
});

describe('BookCancelPage link status (resolveLink 404 vs 410)', () => {
  it('shows the expired-link state on a 410, not the not-found copy', async () => {
    getCancelInfo.mockRejectedValue(await makeConsumedHttpError(410, { error: 'link expired' }));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(ES_AR.publicBooking.linkExpired)).toBeInTheDocument();
    });
    expect(screen.queryByText(ES_AR.publicBooking.invalidCancelLink)).not.toBeInTheDocument();
  });

  it('shows the not-found copy on a 404, not the expired-link state', async () => {
    getCancelInfo.mockRejectedValue(await makeConsumedHttpError(404, { error: 'unknown token' }));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(ES_AR.publicBooking.invalidCancelLink)).toBeInTheDocument();
    });
    expect(screen.queryByText(ES_AR.publicBooking.linkExpired)).not.toBeInTheDocument();
  });

  it('shows a retryable error, not the invalid-link dead end, on a 500', async () => {
    getCancelInfo.mockRejectedValue(await makeConsumedHttpError(500, { error: 'boom' }));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(ES_AR.publicBooking.cancelInfoLoadError)).toBeInTheDocument();
    });
    expect(screen.queryByText(ES_AR.publicBooking.invalidCancelLink)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: ES_AR.publicBooking.tryAgain })).toBeInTheDocument();
  });

  it('retries getCancelInfo when the retry button is clicked after a 500', async () => {
    getCancelInfo.mockRejectedValueOnce(await makeConsumedHttpError(500, { error: 'boom' }));
    getCancelInfo.mockResolvedValueOnce(mockCancelInfo);
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(ES_AR.publicBooking.cancelInfoLoadError)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.tryAgain }));

    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });
  });
});

describe('BookCancelPage refund method (manual must not look like automatic, before confirming)', () => {
  it('shows the manual-handover copy for refund_method "manual", not the automatic-amount copy', async () => {
    getCancelInfo.mockResolvedValue({ ...mockCancelInfo, refund_method: 'manual' });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    expect(screen.getByText(ES_AR.publicBooking.refundManualDescription)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.publicBooking.refundRowLabel)).not.toBeInTheDocument();
  });

  // `refund_amount` only comes back set for an automatic (mercadopago)
  // refund the platform will actually issue — a manual refund_method leaves
  // it unset, which is why the test above doesn't set it.
  it('shows the automatic-refund amount sentence when cancel-info answers with refund_amount, not the manual copy', async () => {
    getCancelInfo.mockResolvedValue({
      ...mockCancelInfo,
      refund_method: 'mercadopago',
      refund_amount: 500000,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    expect(screen.getByText(ES_AR.publicBooking.refundRowLabel).closest('div')).toHaveTextContent('5.000');
    expect(screen.queryByText(ES_AR.publicBooking.refundManualDescription)).not.toBeInTheDocument();
  });

  it('shows the "nothing to refund" copy when refund_method is "none", not the automatic or manual copy', async () => {
    getCancelInfo.mockResolvedValue({
      ...mockCancelInfo,
      can_refund: false,
      refund_method: 'none',
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    expect(screen.queryByText(ES_AR.publicBooking.paidLabel)).not.toBeInTheDocument();
    expect(screen.queryByText(ES_AR.publicBooking.refundRowLabel)).not.toBeInTheDocument();
    expect(screen.queryByText(ES_AR.publicBooking.refundManualDescription)).not.toBeInTheDocument();
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('BookCancelPage refund method — expired window and confirm dialog', () => {
  // Refund window already passed, but there was still money paid — the
  // subtitle names the amount and RefundExpiredNotice below still carries
  // the cancellation-window explanation.
  it('shows the "paid but no longer refundable" amount sentence when can_refund is false and paid_amount > 0', async () => {
    getCancelInfo.mockResolvedValue({
      ...mockCancelInfo,
      can_refund: false,
      refund_method: 'mercadopago',
      paid_amount: 680000,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    expect(screen.getByText(ES_AR.publicBooking.paidLabel).closest('div')).toHaveTextContent('6.800');
    expect(screen.getByText(ES_AR.publicBooking.refundExpired)).toBeInTheDocument();
  });

  it('carries the manual-vs-automatic distinction into the confirm dialog', async () => {
    getCancelInfo.mockResolvedValue({ ...mockCancelInfo, refund_method: 'manual' });
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    expect(screen.getByText(ES_AR.publicBooking.confirmCancelRefundManualDetail)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.publicBooking.confirmCancelRefundDetail)).not.toBeInTheDocument();
  });
});

describe('BookCancelPage cancellation outcome renders the server refund.message', () => {
  it('renders refund.message verbatim for a "manual" outcome', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    cancelBooking.mockResolvedValue({
      refunded: false,
      refund: {
        status: 'manual',
        message: 'El complejo tiene que devolverte este dinero a mano. Comunicate con ellos.',
      },
    });
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.confirmCancelYes }));

    await waitFor(() => {
      expect(
        screen.getByText('El complejo tiene que devolverte este dinero a mano. Comunicate con ellos.'),
      ).toBeInTheDocument();
    });
  });

  it('renders a different refund.message for a different outcome ("issued"), proving it is not a hardcoded string', async () => {
    getCancelInfo.mockResolvedValue(mockCancelInfo);
    cancelBooking.mockResolvedValue({
      refunded: true,
      refund: {
        status: 'issued',
        message: 'La devolución fue enviada a MercadoPago y se acredita en los próximos días hábiles.',
        amount: 500000,
      },
    });
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    await user.click(screen.getByRole('button', { name: ES_AR.publicBooking.confirmCancelYes }));

    await waitFor(() => {
      expect(
        screen.getByText('La devolución fue enviada a MercadoPago y se acredita en los próximos días hábiles.'),
      ).toBeInTheDocument();
    });
    expect(
      screen.queryByText('El complejo tiene que devolverte este dinero a mano. Comunicate con ellos.'),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/\$\s*5\.?000/)).toBeInTheDocument();
  });
});
