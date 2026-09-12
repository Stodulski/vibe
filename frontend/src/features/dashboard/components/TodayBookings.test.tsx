import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodayBookings } from './TodayBookings';
import { renderWithProviders } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import type { Booking } from '@/shared/types/api.types';

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

describe('TodayBookings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders section with aria label', () => {
    renderWithProviders(<TodayBookings bookings={[]} complexId="c1" />);
    expect(screen.getByLabelText(/pr.ximas reservas/i)).toBeInTheDocument();
  });

  it('renders empty state when no bookings', () => {
    renderWithProviders(<TodayBookings bookings={[]} complexId="c1" />);
    expect(screen.getByText(/no hay reservas pr.ximas/i)).toBeInTheDocument();
  });

  it('renders booking details', () => {
    const bookings = [createBooking('b1', { client_name: 'Juan Perez', court_name: 'Cancha 3' })];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" />);
    // The row prints the name twice — once for the mobile layout, once for
    // the desktop one — so both copies are expected, not a duplicate render.
    expect(screen.getAllByText(/Juan Perez/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Cancha 3/).length).toBeGreaterThan(0);
  });

  it('renders booking count badge', () => {
    const bookings = [createBooking('b1'), createBooking('b2')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" />);
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('renders view all buttons', () => {
    const bookings = [createBooking('b1')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" />);
    expect(screen.getAllByRole('button', { name: /ver todas/i })).not.toHaveLength(0);
  });

  it('navigates to bookings page when view all is clicked', async () => {
    const user = userEvent.setup();
    const bookings = [createBooking('b1')];
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" />);
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
    renderWithProviders(<TodayBookings bookings={bookings} complexId="c1" />);
    expect(screen.queryByText(/pendiente/i)).not.toBeInTheDocument();
    expect(screen.getAllByText(/Confirmed Client/).length).toBeGreaterThan(0);
  });
});
