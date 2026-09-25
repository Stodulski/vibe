import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodayBookings } from './TodayBookings';
import { renderWithProviders } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import type { Booking, PaymentSummary } from '@/shared/types/api.types';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function createBooking(id: string, overrides?: Partial<Booking>): Booking {
  return makeBooking({
    id,
    client_name: `Cliente ${id}`,
    date: '2026-03-16',
    start_time: '10:00',
    duration_minutes: 90,
    price: 1500000,
    notes: '',
    ...overrides,
  });
}

// Every test but the ones exercising the summary metrics themselves only
// cares about the booking list, so the moved-in metrics default to values
// that render but assert nothing on their own (0 bookings, no comparison).
const defaultSummaryProps = {
  todayBookingsCount: 0,
  yesterdayBookingsCount: 0,
  occupancyRate: 0,
  statusEntries: [] as [string, PaymentSummary['by_status'][string]][],
};

describe('TodayBookings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders section with aria label', () => {
    renderWithProviders(<TodayBookings bookings={[]} complexId="c1" {...defaultSummaryProps} />);
    expect(screen.getByLabelText(/pr.ximas reservas/i)).toBeInTheDocument();
  });

  it('renders empty state when no bookings', () => {
    renderWithProviders(<TodayBookings bookings={[]} complexId="c1" {...defaultSummaryProps} />);
    expect(screen.getByText(/no hay reservas pr.ximas/i)).toBeInTheDocument();
  });

  it('renders booking details', () => {
    const bookings = [createBooking('b1', { client_name: 'Juan Perez', court_name: 'Cancha 3' })];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" {...defaultSummaryProps} />);
    // The row prints the name twice — once for the mobile layout, once for
    // the desktop one — so both copies are expected, not a duplicate render.
    expect(screen.getAllByText(/Juan Perez/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Cancha 3/).length).toBeGreaterThan(0);
  });

  it('renders booking count badge', () => {
    const bookings = [createBooking('b1'), createBooking('b2')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" {...defaultSummaryProps} />);
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('renders view all buttons', () => {
    const bookings = [createBooking('b1')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" {...defaultSummaryProps} />);
    expect(screen.getAllByRole('button', { name: /ver todas/i })).not.toHaveLength(0);
  });

  it('navigates to bookings page when view all is clicked', async () => {
    const user = userEvent.setup();
    const bookings = [createBooking('b1')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" {...defaultSummaryProps} />);
    for (const button of screen.getAllByRole('button', { name: /ver todas/i })) {
      await user.click(button);
    }
    expect(mockNavigate).toHaveBeenCalledWith('/bookings');
  });

  it('filters out pending bookings — they are waiting on the client, not the owner', () => {
    const bookings = [
      createBooking('b1', {
        status: 'pending',
        collection_status: 'unpaid',
        refund_status: 'none',
      }),
      createBooking('b2', {
        status: 'confirmed',
        collection_status: 'unpaid',
        refund_status: 'none',
        client_name: 'Confirmed Client',
      }),
    ];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" {...defaultSummaryProps} />);
    expect(screen.queryByText(/pendiente/i)).not.toBeInTheDocument();
    expect(screen.getAllByText(/Confirmed Client/).length).toBeGreaterThan(0);
  });
});

// ─── Summary metrics moved in from PaymentOverview (dashboard-today-card) ───
describe('TodayBookings — summary metrics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders today's booking count and the occupancy rate", () => {
    renderWithProviders(
      <TodayBookings
        bookings={[]}
        complexId="c1"
        {...defaultSummaryProps}
        todayBookingsCount={5}
        yesterdayBookingsCount={2}
        occupancyRate={42}
      />,
    );
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('42%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42');
  });

  it('renders the payment-status breakdown moved in from PaymentOverview', () => {
    renderWithProviders(
      <TodayBookings
        bookings={[]}
        complexId="c1"
        {...defaultSummaryProps}
        statusEntries={[['fully_paid', { count: 2, total: 300000 }]]}
      />,
    );
    expect(screen.getByText('Estado de cobro')).toBeInTheDocument();
    expect(screen.getByText('Pagado')).toBeInTheDocument();
  });
});
