import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingConfirmed } from './BookingConfirmed';
import { baseProps } from './bookingConfirmedFixtures';

const defaultProps = { ...baseProps, onRetry: vi.fn(), onStatusRetry: vi.fn() };

beforeEach(() => {
  vi.clearAllMocks();
});

describe('BookingConfirmed success state', () => {
  it('renders success state with booking details', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByText(/reserva confirmada/i)).toBeInTheDocument();
  });

  it('shows court name in success state', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
  });

  it('shows booking time range', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByText(/10:00 a 11:30/)).toBeInTheDocument();
  });

  // The server never sends the booking's id (specs/booking-link-credential);
  // the token that replaced it is a live bearer credential and must never be
  // shown, even truncated — showing or copying a prefix of it is the exact
  // leak class the server change closes. There is deliberately no booking
  // reference display any more.
  it('never displays the token, even truncated, as a booking reference', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.queryByText(/TOK-ABC12345/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/reserva:/i)).not.toBeInTheDocument();
  });

  it('shows the balance owed at the venue once, in the money summary', () => {
    render(<BookingConfirmed {...defaultProps} />);
    // remaining = 1500000 - 500000 = 1000000 = $10.000, no serviceFee in the
    // fixture so it reads "Pagaste ... / Resta pagar $10.000."
    expect(screen.getByText('Resta pagar').closest('div')).toHaveTextContent('10.000');
    expect(screen.queryByText(/recordá/i)).not.toBeInTheDocument();
  });

  it('shows no action buttons on the confirmed screen any more — no calendar, no WhatsApp', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.queryByText(/agregar al calendario/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/whatsapp/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/google calendar/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/compartir/i)).not.toBeInTheDocument();
  });

  it('never renders a tel: or wa.me link (there are no contact actions on this screen)', () => {
    render(<BookingConfirmed {...defaultProps} />);
    const contactLinks = screen.queryAllByRole('link').filter((a) => {
      const href = a.getAttribute('href') ?? '';
      return href.startsWith('tel:') || href.includes('wa.me');
    });
    expect(contactLinks).toHaveLength(0);
  });

  it('shows make another booking button', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByText(/nueva reserva/i)).toBeInTheDocument();
  });

  it('shows cancel button when status is confirmed', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByRole('button', { name: /cancelar reserva/i })).toBeInTheDocument();
  });

  it('shows exactly one filled (primary) button on the confirmed screen: "Nueva reserva"', () => {
    render(<BookingConfirmed {...defaultProps} />);
    const filled = screen.getAllByRole('button').filter((btn) => btn.dataset.variant === 'default');
    expect(filled).toHaveLength(1);
    expect(filled[0]).toHaveTextContent(/nueva reserva/i);
  });

  it('shows cancellation hours info', () => {
    render(<BookingConfirmed {...defaultProps} />);
    expect(screen.getByText(/24h antes/)).toBeInTheDocument();
  });
});

describe('BookingConfirmed loading/pending states', () => {
  it('renders loading state', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'loading' }} status={undefined} collectionStatus={undefined} />,
    );
    expect(screen.getByText(/procesando/i)).toBeInTheDocument();
  });

  it('renders pending polling state', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'pending' }} status="pending" collectionStatus="unpaid" />,
    );
    expect(screen.getByText(/procesando/i)).toBeInTheDocument();
  });

  // A screen reader needs to be told when this silently repaints (pending ->
  // under review, or -> timed out) since nothing else on screen announces it.
  it('announces the pending state through a polite live region', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'pending' }} status="pending" collectionStatus="unpaid" />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(/procesando/i);
  });
});

describe('BookingConfirmed cancelled state — rejected payment (collection_status stays "unpaid")', () => {
  it('renders the payment-failed copy', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'cancelled' }} status="cancelled" collectionStatus="unpaid" />,
    );
    // "Pago rechazado"
    expect(screen.getByText(/pago rechazado/i)).toBeInTheDocument();
  });

  it('renders retry button on failure', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'cancelled' }} status="cancelled" collectionStatus="unpaid" />,
    );
    // "Volver a intentar"
    expect(screen.getByRole('button', { name: /reintentar/i })).toBeInTheDocument();
  });

  it('calls onRetry when retry button is clicked on failure', async () => {
    const user = userEvent.setup();
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'cancelled' }} status="cancelled" collectionStatus="unpaid" />,
    );
    await user.click(screen.getByRole('button', { name: /reintentar/i }));
    expect(defaultProps.onRetry).toHaveBeenCalledTimes(1);
  });
});

describe('BookingConfirmed cancelled state — not a rejected payment (money was collected)', () => {
  // Two values rather than the old five: a refund under way is no longer a
  // value of this field. The collection axis is the whole question here —
  // money arrived, so this cannot be a decline — and the three refund states
  // the old enum spelled here now live in refund_status, which this screen
  // does not read at all. The case they covered is the one below.
  it.each(['deposit_paid', 'fully_paid'] as const)(
    'renders neutral cancellation copy, not the card-declined copy, when collection_status is %s',
    (collectionStatus) => {
      render(
        <BookingConfirmed
          {...defaultProps}
          view={{ kind: 'cancelled' }}
          status="cancelled"
          collectionStatus={collectionStatus}
        />,
      );
      expect(screen.getByText(/tu reserva fue cancelada/i)).toBeInTheDocument();
      expect(screen.queryByText(/no pudimos procesar/i)).not.toBeInTheDocument();
    },
  );

  it('mentions a refund rather than asking the person to try their card again', () => {
    render(
      <BookingConfirmed
        {...defaultProps}
        view={{ kind: 'cancelled' }}
        status="cancelled"
        collectionStatus="deposit_paid"
      />,
    );
    expect(screen.getByText(/te lo devolvemos/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /nueva reserva/i })).toBeInTheDocument();
  });
});

describe('BookingConfirmed timeout state', () => {
  it('renders timeout state', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'timed_out' }} status="pending" collectionStatus="unpaid" />,
    );
    // "Pago en proceso"
    expect(screen.getByText(/pago en proceso/i)).toBeInTheDocument();
  });

  it('shows complex phone in timeout state', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'timed_out' }} status="pending" collectionStatus="unpaid" />,
    );
    expect(screen.getByText('+5491155550000')).toBeInTheDocument();
  });

  it('announces the timeout state through a polite live region', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'timed_out' }} status="pending" collectionStatus="unpaid" />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(/pago en proceso/i);
  });
});

describe('BookingConfirmed link status (resolveLink 404 vs 410)', () => {
  it('renders the expired-link state', () => {
    render(
      <BookingConfirmed
        {...defaultProps}
        view={{ kind: 'link_expired' }}
        status={undefined}
        collectionStatus={undefined}
      />,
    );
    expect(screen.getByText(/este link venci./i)).toBeInTheDocument();
  });

  it('renders the not-found state, not the expired-link one, for a 404 (a different situation)', () => {
    render(
      <BookingConfirmed
        {...defaultProps}
        view={{ kind: 'link_not_found' }}
        status={undefined}
        collectionStatus={undefined}
      />,
    );
    expect(screen.queryByText(/este link venci./i)).not.toBeInTheDocument();
    expect(screen.getByText(/reserva no encontrada/i)).toBeInTheDocument();
  });
});

describe('BookingConfirmed payment-under-review state (MP back_url ?status=pending)', () => {
  it('renders the under-review message instead of the generic timeout copy', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'under_review' }} status="pending" collectionStatus="unpaid" />,
    );
    // "Pago en revisión" — not TimeoutState's "a few minutes" copy.
    expect(screen.getByText(/pago en revisi.n/i)).toBeInTheDocument();
    expect(screen.queryByText(/pago en proceso/i)).not.toBeInTheDocument();
  });

  it('announces the under-review state through a polite live region', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'under_review' }} status="pending" collectionStatus="unpaid" />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(/pago en revisi.n/i);
  });
});

// M2/M11: the request itself can fail (a network drop or a 500) independent
// of anything the booking or payment did. Before this state existed, that
// case fell through to `LoadingState` (and eventually `TimeoutState`), which
// told the visitor their payment was "processing" when the real problem was
// the status check never getting an answer at all.
describe('BookingConfirmed error state (GET /book/status failed, no prior data)', () => {
  it('renders the status-error copy, not the loading copy', () => {
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'error' }} status={undefined} collectionStatus={undefined} />,
    );
    expect(screen.getByText(/no pudimos consultar el estado de tu pago/i)).toBeInTheDocument();
    expect(screen.queryByText(/procesando/i)).not.toBeInTheDocument();
  });

  it('calls onStatusRetry, not onRetry, when its retry button is clicked', async () => {
    const user = userEvent.setup();
    render(
      <BookingConfirmed {...defaultProps} view={{ kind: 'error' }} status={undefined} collectionStatus={undefined} />,
    );
    await user.click(screen.getByRole('button', { name: /reintentar/i }));
    expect(defaultProps.onStatusRetry).toHaveBeenCalledTimes(1);
    expect(defaultProps.onRetry).not.toHaveBeenCalled();
  });
});
