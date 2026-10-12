import { describe, it, expect, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import { format } from 'date-fns/format';
import { addDays } from 'date-fns/addDays';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { BOOKING_MESSAGES } from '../messages';
import { todayInVenueTz } from '../lib/venueDate';
import type { CourtAvailability, DurationMinutes } from '../api/types';
import ComplexPage from './ComplexPage';

const t = BOOKING_MESSAGES;

// Two days out, from the real clock: the deep link and the strip only accept
// days inside the booking horizon, and a fixed date would go stale.
const day = addDays(todayInVenueTz(), 2);
const dayStr = format(day, 'yyyy-MM-dd');
const nextDayStr = format(addDays(day, 3), 'yyyy-MM-dd');

function court(sport: string, free: boolean): CourtAvailability {
  return {
    court_id: `${sport}-1`,
    court_name: `Cancha de ${sport}`,
    sport,
    court_type: 'outdoor',
    slots: [
      {
        start_time: '18:00',
        end_time: '19:30',
        start_min: 1080,
        duration_minutes: 90,
        price: 1_000_000,
        available: free,
      },
    ],
  };
}

/** The free courts the grid gets for each duration of the chosen day. */
function serveAvailability(courtsByDuration: Partial<Record<DurationMinutes, CourtAvailability[]>>) {
  server.use(
    http.get('*/public/complexes/:slug/availability', ({ request }) => {
      const duration = Number(new URL(request.url).searchParams.get('duration')) as DurationMinutes;
      return HttpResponse.json({
        availability: {
          date: dayStr,
          day: 'monday',
          is_open: true,
          courts: courtsByDuration[duration] ?? [],
        },
      });
    }),
    http.get('*/public/complexes/:slug/next-available', () => HttpResponse.json({ date: nextDayStr })),
  );
}

describe('ComplexPage — a day with nothing to offer', () => {
  beforeEach(() => {
    setBookingUrl(`/c/club-norte?date=${dayStr}&duration=90`);
  });

  it('says the day is full when no court has a free slot at any duration', async () => {
    serveAvailability({ 60: [], 90: [], 120: [] });
    renderBooking(<ComplexPage slug="club-norte" />);

    expect(await screen.findByText(t.publicBooking.fullDayTitle)).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.filterHint)).not.toBeInTheDocument();
  });

  it('hints at the duration instead of calling the day full when another duration has a free slot', async () => {
    serveAvailability({ 60: [court('padel', true)], 90: [], 120: [] });
    renderBooking(<ComplexPage slug="club-norte" />);

    expect(await screen.findByText(t.publicBooking.filterHint)).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.fullDayTitle)).not.toBeInTheDocument();
  });

  it('hints at the sport instead of calling the day full when another sport has a free slot', async () => {
    setBookingUrl(`/c/club-norte?date=${dayStr}&duration=90&sport=padel`);
    serveAvailability({ 90: [court('tennis', true), court('padel', false)] });
    renderBooking(<ComplexPage slug="club-norte" />);

    expect(await screen.findByText(t.publicBooking.filterHint)).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.fullDayTitle)).not.toBeInTheDocument();
  });
});
