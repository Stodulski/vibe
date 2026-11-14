import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingCalendar } from './BookingCalendar';
import { makeBooking } from '@/test/factories';
import { venueInstant } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import type { Booking, CourtWithPrices, Schedule, BlockedSlot, FixedBlock, EventBlock } from '@/shared/types/api.types';

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
    has_lighting: false,
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

describe('BookingCalendar', () => {
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

describe('BookingCalendar — fixed weekly bookings without a booking yet', () => {
  const fixedBlock: FixedBlock = {
    series_id: 'ser1',
    court_id: 'ct1',
    date: futureDate,
    start_time: '19:00',
    end_time: '20:30',
    duration_minutes: 90,
    client_name: 'Ana Gomez',
  };

  it('draws the block labelled as a fixed weekly booking and hands it over on click', async () => {
    const user = userEvent.setup();
    const onSelectFixedBlock = vi.fn();
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        fixedBlocks={[fixedBlock]}
        onSelectFixedBlock={onSelectFixedBlock}
      />,
    );

    const block = screen.getByRole('button', { name: /Bloqueo por turno fijo · Ana Gomez/ });
    expect(block).toHaveTextContent('Bloqueo por turno fijo');
    expect(block).toHaveTextContent('Ana Gomez');
    await user.click(block);

    expect(onSelectFixedBlock).toHaveBeenCalledWith(fixedBlock);
  });

  it('draws hours that run past midnight down to the end of the day instead of dropping them', () => {
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        fixedBlocks={[{ ...fixedBlock, start_time: '23:00', end_time: '00:30', duration_minutes: 90 }]}
      />,
    );

    // 23:00 to the end of the day is 4% of the grid; a backwards range would not be drawn at all.
    const block = screen.getByRole('button', { name: /Bloqueo por turno fijo · Ana Gomez/ });
    expect(block.style.top).toContain('95.83');
  });

  it('offers no free slot under the block', () => {
    render(<BookingCalendar {...defaultProps} bookings={[]} fixedBlocks={[fixedBlock]} />);

    expect(screen.queryByRole('button', { name: /Crear reserva · Cancha 1 · 19:00/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Crear reserva · Cancha 1 · 18:00/ })).toBeInTheDocument();
  });
});

describe('BookingCalendar — open-play events (canchas libres)', () => {
  // 2027-03-20 is a Saturday.
  const eventBlock: EventBlock = {
    event_id: 'ev1',
    court_id: 'ct1',
    date: futureDate,
    start_time: '19:00',
    end_time: '21:00',
    duration_minutes: 120,
    name: 'Americano de sabado',
  };

  it('draws the block as "Canchas libres · {día} {hora}" and hands it over on click', async () => {
    const user = userEvent.setup();
    const onSelectEventBlock = vi.fn();
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        eventBlocks={[eventBlock]}
        onSelectEventBlock={onSelectEventBlock}
      />,
    );

    const block = screen.getByRole('button', { name: /Canchas libres · sábado 19:00/ });
    expect(block).toHaveTextContent('Canchas libres · sábado 19:00');
    expect(block).toHaveTextContent('Americano de sabado');
    await user.click(block);

    expect(onSelectEventBlock).toHaveBeenCalledWith(eventBlock);
  });

  it('draws hours that run past midnight down to the end of the day', () => {
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        eventBlocks={[{ ...eventBlock, start_time: '23:00', end_time: '01:00', duration_minutes: 120 }]}
      />,
    );

    const block = screen.getByRole('button', { name: /Canchas libres · sábado 23:00/ });
    expect(block.style.top).toContain('95.83');
  });

  it('offers no free slot under the block', () => {
    render(<BookingCalendar {...defaultProps} bookings={[]} eventBlocks={[eventBlock]} />);

    expect(screen.queryByRole('button', { name: /Crear reserva · Cancha 1 · 19:00/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Crear reserva · Cancha 1 · 20:30/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Crear reserva · Cancha 1 · 18:00/ })).toBeInTheDocument();
  });

  it('draws nothing on a court the event does not hold', () => {
    render(<BookingCalendar {...defaultProps} bookings={[]} eventBlocks={[{ ...eventBlock, court_id: 'other' }]} />);
    expect(screen.queryByRole('button', { name: /Canchas libres/ })).not.toBeInTheDocument();
  });
});
