import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingConfirmed } from './BookingConfirmed';
import { ES_AR } from '@/shared/i18n/es_AR';
import { mockBookingInfo, baseProps } from './bookingConfirmedFixtures';
import type { BookingStatusDetails } from '@/shared/types/api.types';

const t = ES_AR;

const defaultProps = { ...baseProps, onRetry: vi.fn(), onStatusRetry: vi.fn() };

// The full GET /book/status payload once the server agent's extension ships —
// every field the success page now renders straight from the API,
// independent of whatever sits in the sessionStorage cache.
const mockBookingDetails: BookingStatusDetails = {
  status: 'confirmed',
  collection_status: 'deposit_paid',
  refund_status: 'none',
  complex_name: 'Club API',
  complex_address: 'Av. Siempre Viva 123',
  complex_phone: '+5491100001111',
  court_name: 'Cancha API',
  sport: 'padel',
  court_type: 'indoor',
  date: '2026-03-15',
  start_time: '09:00',
  starts_at: '2026-03-15T09:00:00-03:00',
  ends_at: '2026-03-15T10:30:00-03:00',
  duration_minutes: 90,
  price: 2000000,
  deposit_amount: 600000,
  service_fee: 80000,
  remaining_amount: 1320000,
  cancellation: {
    can_cancel: true,
    refund_deadline: '2026-03-15T10:00:00-03:00',
    can_refund_now: true,
    cancellation_hours: 12,
  },
};

// What an older, not-yet-restarted server still answers with — only the two
// fields this success page originally had, nothing this change added.
const oldApiBookingDetails: BookingStatusDetails = {
  status: 'confirmed',
  collection_status: 'deposit_paid',
  refund_status: 'none',
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('BookingConfirmed success state — booking details from GET /book/status', () => {
  it('renders every detail straight from the API with no cache at all', () => {
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={mockBookingDetails} />);
    // Complex name only — no address, and no contact action renders the
    // phone as a tel: link either.
    expect(screen.getByText(/club api/i)).not.toHaveTextContent('Av. Siempre Viva 123');
    expect(screen.queryAllByRole('link').some((a) => a.getAttribute('href')?.startsWith('tel:'))).toBe(false);
    expect(screen.getByText('Cancha API')).toBeInTheDocument();
    expect(screen.getByText('Pádel')).toBeInTheDocument();
    expect(screen.getByText('Techada')).toBeInTheDocument();
    expect(screen.getByText(/09:00 a 10:30/)).toBeInTheDocument();
    // deposit 600000 + serviceFee 80000 = 680000 = $6.800 already paid;
    // remaining 1320000 = $13.200 still owed, in the bold clause.
    expect(screen.getByText(t.publicBooking.paidLabel).closest('div')).toHaveTextContent('6.800');
    expect(screen.getByText(t.publicBooking.oweAtClubLabel).closest('div')).toHaveTextContent('13.200');
  });

  it('falls back to the sessionStorage cache when the API answer predates these fields (older server)', () => {
    render(<BookingConfirmed {...defaultProps} bookingInfo={mockBookingInfo} bookingDetails={oldApiBookingDetails} />);
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    expect(screen.getByText(/10:00 a 11:30/)).toBeInTheDocument();
    // No service fee in the cache — the sentence names only the deposit.
    expect(screen.getByText(t.publicBooking.paidLabel)).toBeInTheDocument();
    // remaining falls back to price - deposit, same as before this change.
    expect(screen.getByText(t.publicBooking.oweAtClubLabel).closest('div')).toHaveTextContent('10.000');
  });

  it('renders no details block, but does not crash, with neither a cache nor API fields', () => {
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={oldApiBookingDetails} />);
    expect(screen.getByText(/reserva confirmada/i)).toBeInTheDocument();
    expect(screen.queryByText(t.bookings.court)).not.toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.paidLabel)).not.toBeInTheDocument();
  });
});

describe('BookingConfirmed cancellation copy — GET /book/status "cancellation"', () => {
  it('shows the refund-deadline sentence when refundable now with a deadline', () => {
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={mockBookingDetails} />);
    expect(screen.getByText(/cancelación gratis hasta el.*15 de marzo, 10:00\./i)).toBeInTheDocument();
  });

  it('shows the "antes de que empiece el turno" sentence when refundable now with no fixed deadline', () => {
    const details: BookingStatusDetails = {
      ...mockBookingDetails,
      cancellation: {
        can_cancel: true,
        refund_deadline: null,
        can_refund_now: true,
        cancellation_hours: 12,
      },
    };
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={details} />);
    expect(screen.getByText(/cancelación gratis hasta que empiece el turno\./i)).toBeInTheDocument();
  });

  it('shows the window-passed sentence when cancellable but no longer refundable', () => {
    const details: BookingStatusDetails = {
      ...mockBookingDetails,
      cancellation: {
        can_cancel: true,
        refund_deadline: null,
        can_refund_now: false,
        cancellation_hours: 12,
      },
    };
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={details} />);
    expect(screen.getByText(/ya no podés cancelar gratis/i)).toBeInTheDocument();
  });

  it('hides both the cancellation line and the cancel button when can_cancel is false', () => {
    const details: BookingStatusDetails = {
      ...mockBookingDetails,
      cancellation: {
        can_cancel: false,
        refund_deadline: null,
        can_refund_now: false,
        cancellation_hours: 12,
      },
    };
    render(<BookingConfirmed {...defaultProps} bookingInfo={null} bookingDetails={details} />);
    expect(screen.queryByRole('button', { name: /cancelar reserva/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/ya pasó/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/cancelar gratis/i)).not.toBeInTheDocument();
  });
});
