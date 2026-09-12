import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingCalendar } from './BookingCalendar';
import { makeBooking } from '@/test/factories';
import { venueInstant } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import type { Booking, CourtWithPrices, Schedule, BlockedSlot } from '@/shared/types/api.types';

const futureDate = '2027-03-20';

/**
 * The instant a wall-clock reading names in the venue's own zone
 * (`VENUE_TIME_ZONE`), never the test runner's. Booking blocks read their
 * displayed hours from `formatHourRange`, which always formats in the venue
 * zone, so a mock booking left to derive its `starts_at` from
 * `makeBooking`'s own default (built from the runner's zone) would name a
 * different real moment — and show a different hour — on any machine or CI
 * runner not itself set to Argentina time.
 */
function venueTime(date: string, hhmm: string): string {
  const instant = venueInstant(date, timeToMinutes(hhmm));
  if (!instant) throw new Error(`invalid fixture date/time: ${date} ${hhmm}`);
  return instant;
}

const mockCourts: CourtWithPrices[] = [
  {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  },
];

const mockSchedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'saturday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's2',
    complex_id: 'c1',
    day: 'sunday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's3',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's4',
    complex_id: 'c1',
    day: 'tuesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's5',
    complex_id: 'c1',
    day: 'wednesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's6',
    complex_id: 'c1',
    day: 'thursday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's7',
    complex_id: 'c1',
    day: 'friday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
];

const mockBookings: Booking[] = [
  makeBooking({
    id: 'b1',
    complex_id: 'c1',
    court_id: 'ct1',
    client_id: 'cl1',
    date: futureDate,
    start_time: '10:00',
    duration_minutes: 90,
    starts_at: venueTime(futureDate, '10:00'),
    price: 1500000,
    deposit_amount: 0,
    status: 'confirmed',
    collection_status: 'unpaid',
    refund_status: 'none',
    notes: null,
    court_name: 'Cancha 1',
    client_name: 'Juan Perez',
    client_phone: '1155550000',
    reminder_sent_2h: false,
    created_at: '2026-03-18T10:00:00Z',
    updated_at: '2026-03-18T10:00:00Z',
  }),
];

describe('BookingCalendar', () => {
  const defaultProps = {
    bookings: mockBookings,
    blockedSlots: [] as BlockedSlot[],
    courts: mockCourts,
    date: futureDate,
    schedules: mockSchedules,
    onSelectBooking: vi.fn(),
    onCreateBooking: vi.fn(),
    onSelectBlockedSlot: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders booking client name', () => {
    render(<BookingCalendar {...defaultProps} />);
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
  });

  it('renders booking time range', () => {
    render(<BookingCalendar {...defaultProps} />);
    // A plain /10:00/ regex now also matches the grid's hour-gutter label
    // (the vertical grid has its own "10:00" ruler entry), so this asserts
    // the booking block's full range text instead of a loose substring.
    expect(screen.getByText('10:00 – 11:30')).toBeInTheDocument();
  });

  it('renders court name in booking card', () => {
    render(<BookingCalendar {...defaultProps} />);
    const elements = screen.getAllByText('Cancha 1');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('calls onSelectBooking when booking card is clicked', async () => {
    const user = userEvent.setup();
    render(<BookingCalendar {...defaultProps} />);

    // The block is a real <button> now (A11Y-02), not a div with role.
    const bookingCard = screen.getByText('Juan Perez').closest('button');
    if (!bookingCard) throw new Error('booking card not found');
    await user.click(bookingCard);

    expect(defaultProps.onSelectBooking).toHaveBeenCalledWith(mockBookings[0]);
  });

  it('renders closed message when day is closed', () => {
    const closedSchedules = mockSchedules.map((s) => (s.day === 'saturday' ? { ...s, is_closed: true } : s));
    render(<BookingCalendar {...defaultProps} date="2027-03-20" schedules={closedSchedules} />);
    expect(screen.getByText(/cerrado/i)).toBeInTheDocument();
  });

  it('renders empty state when no bookings', () => {
    render(<BookingCalendar {...defaultProps} bookings={[]} />);
    expect(screen.queryByText('Juan Perez')).not.toBeInTheDocument();
  });
});
