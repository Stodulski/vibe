import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingCalendar } from './BookingCalendar';
import type { BlockedSlot, CourtWithPrices, Schedule } from '@/shared/types/api.types';

// 2027-03-20 is a Saturday.
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

const defaultProps = {
  bookings: [],
  blockedSlots: [] as BlockedSlot[],
  courts,
  date,
  schedules,
  onSelectBooking: () => undefined,
  onCreateBooking: () => undefined,
  onSelectBlockedSlot: () => undefined,
};

describe('BookingCalendar — hours that started the evening before', () => {
  const eve = '2027-03-19';
  const tail = { date: eve, start_time: '23:00', end_time: '01:00', duration_minutes: 120 };

  it('draws the tail of a fixed booking from the top of the grid to its end', () => {
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        fixedBlocks={[{ series_id: 'ser1', court_id: 'ct1', client_name: 'Ana Gomez', ...tail }]}
      />,
    );

    // 00:00 to 01:00 is 1/24 of the grid, from the very top.
    const block = screen.getByRole('button', { name: /Turno fijo · Ana Gomez/ });
    expect(block.style.top).toContain('calc(0%');
    expect(block.style.height).toContain('4.16');
    expect(screen.queryByRole('button', { name: /Crear reserva · Cancha 1 · 00:30/ })).not.toBeInTheDocument();
  });

  it('draws the tail of an event and of a tournament match', () => {
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        eventBlocks={[
          { event_id: 'ev1', court_id: 'ct1', name: 'Americano', ...tail },
          { match_id: 'm1', tournament_id: 't1', court_id: 'ct1', name: 'Copa Vibe · 6ta', ...tail },
        ]}
      />,
    );

    for (const name of [/Canchas libres · viernes 23:00/, /Torneo · Copa Vibe · 6ta/]) {
      const block = screen.getByRole('button', { name });
      expect(block.style.top).toContain('calc(0%');
      expect(block.style.height).toContain('4.16');
    }
  });

  it('does not repeat the evening part of a block that started the day before', () => {
    render(
      <BookingCalendar
        {...defaultProps}
        bookings={[]}
        fixedBlocks={[{ series_id: 'ser1', court_id: 'ct1', client_name: 'Ana Gomez', ...tail }]}
      />,
    );
    expect(screen.getAllByRole('button', { name: /Turno fijo · Ana Gomez/ })).toHaveLength(1);
  });
});

const hour = (start: string, end: string) => ({
  court_id: 'ct1',
  date,
  start_time: start,
  end_time: end,
  duration_minutes: 60,
});

describe('BookingCalendar — what each kind of hours is drawn in', () => {
  it('keeps the error red for manual blocks and gives fixed bookings, events and matches their own colour', () => {
    const blocked: BlockedSlot = {
      ...hour('10:00', '11:00'),
      id: 'b1',
      court_name: 'Cancha 1',
      created_at: '2026-01-01T00:00:00Z',
    };
    render(
      <BookingCalendar
        {...defaultProps}
        blockedSlots={[blocked]}
        fixedBlocks={[{ ...hour('12:00', '13:00'), series_id: 's', client_name: 'Ana' }]}
        eventBlocks={[
          { ...hour('14:00', '15:00'), event_id: 'e', name: 'Libres' },
          { ...hour('16:00', '17:00'), match_id: 'm', tournament_id: 't', name: 'Copa' },
        ]}
      />,
    );

    const fill = (name: RegExp) => screen.getByRole('button', { name }).className;
    expect(fill(/Horario bloqueado/)).toContain('bg-error-border');
    const kinds = [/Turno fijo · Ana/, /Canchas libres/, /Torneo · Copa/];
    for (const name of kinds) expect(fill(name)).not.toContain('error');
    expect(new Set(kinds.map((name) => /bg-[\w/-]+/.exec(fill(name))?.[0])).size).toBe(3);
  });
});
