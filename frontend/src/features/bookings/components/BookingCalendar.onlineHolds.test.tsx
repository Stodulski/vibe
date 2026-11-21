import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingCalendar } from './BookingCalendar';
import type { BlockedSlot, CourtWithPrices, Schedule } from '@/shared/types/api.types';

// 2027-03-20 is a Saturday, far enough ahead that every hour is still bookable.
const date = '2027-03-20';

const courts: CourtWithPrices[] = [
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

const schedules: Schedule[] = [
  { id: 's1', complex_id: 'c1', day: 'saturday', open_time: '08:00', close_time: '23:00', is_closed: false },
];

const props = {
  bookings: [],
  blockedSlots: [] as BlockedSlot[],
  courts,
  date,
  schedules,
  onSelectBooking: () => undefined,
  onCreateBooking: () => undefined,
};

// 22:42 UTC is 19:42 in Buenos Aires.
const hold = {
  court_id: 'ct1',
  date,
  start_time: '19:00',
  end_time: '20:30',
  duration_minutes: 90,
  frees_at: '2027-03-20T22:42:00Z',
};

// The court name is kept on one line with a no-break space.
const freeSlot = (time: string) =>
  screen.queryByRole('button', { name: new RegExp(`^Crear reserva · Cancha.1 · ${time}$`) });

describe('BookingCalendar, hours a visitor is booking online', () => {
  it('draws them as being booked online, with when they free up, and offers nothing under them', () => {
    render(<BookingCalendar {...props} onlineHolds={[hold]} />);

    const block = screen.getByText('Reservando online').closest('[data-online-hold]');
    expect(block).not.toBeNull();
    expect(block).toHaveTextContent('Libre a las 19:42 si no se paga');
    expect(block?.closest('button')).toBeNull();
    expect(freeSlot('19:00')).toBeNull();
    expect(freeSlot('20:00')).toBeNull();
    // The hours after the hold are still on offer.
    expect(freeSlot('20:30')).not.toBeNull();
  });

  it('leaves the hours free when nobody is booking them', () => {
    render(<BookingCalendar {...props} />);

    expect(screen.queryByText('Reservando online')).not.toBeInTheDocument();
    expect(freeSlot('19:00')).not.toBeNull();
  });
});
