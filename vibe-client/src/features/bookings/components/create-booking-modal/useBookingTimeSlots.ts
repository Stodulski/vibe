import { useMemo } from 'react';
import { generateTimeSlots, timeToMinutes } from '@/shared/lib/time';
import { overlaps, spanOnDay, toSpan } from '@/shared/lib/instants';
import { useBookings } from '../../hooks/useBookings';
import { nowInArgentina, todayInArgentina } from '../../lib/today';

export function useBookingTimeSlots({
  open,
  complexId,
  courtId,
  date,
  durationMinutes,
}: {
  open: boolean;
  complexId: string;
  courtId: string;
  date: string;
  durationMinutes: number;
}) {
  // Fetch existing bookings for the selected date to filter occupied slots.
  const { data: existingBookings } = useBookings(open ? complexId : null, date);

  const timeSlots = useMemo(() => {
    // Staff can book any hour of the day from this dashboard, not just the
    // complex's opening hours — the whole 24h day, on the usual 30-minute
    // step.
    const all = generateTimeSlots('00:00', '24:00');

    // Every start on the grid is offered. A span that runs past midnight used
    // to be dropped here because the server refused it; it now ends on the
    // following day like any other span, and the only thing that removes a
    // start is an actual collision below.
    let filtered = all;

    // Filter past times for today, in the venue's timezone (Argentina) — not
    // the browser's, which drifts from it near midnight (see lib/today.ts).
    if (date && date === todayInArgentina()) {
      const now = nowInArgentina();
      const currentTime = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`;
      filtered = filtered.filter((slot) => slot > currentTime);
    }

    // Filter slots occupied by active bookings for the selected court.
    if (courtId && existingBookings?.length) {
      const courtBookings = existingBookings.filter((b) => b.court_id === courtId && b.status !== 'cancelled');

      // One span against another, on instants.
      //
      // This walked the candidate in half-hour steps and asked, of each step's
      // clock reading, `checkTime >= b.start_time && checkTime < b.end_time`.
      // Against a booking that ran 23:00 to 01:00 that is false for every
      // possible reading — no time is at once past 23:00 and before 01:00 — so
      // the booking read as occupying nothing and the form offered its hours
      // for sale a second time. The stepping was there to catch a booking
      // starting inside the candidate; an overlap test catches that by
      // construction and does not care which day either end fell on.
      const bookedSpans = courtBookings.map((b) => toSpan(b.starts_at, b.ends_at)).filter((span) => span !== null);

      filtered = filtered.filter((slot) => {
        const startMin = timeToMinutes(slot);
        const candidate = spanOnDay(date, startMin, startMin + durationMinutes);
        if (!candidate) return true;
        return !bookedSpans.some((b) => overlaps(candidate.start, candidate.end, b.start, b.end));
      });
    }

    return filtered;
  }, [date, courtId, existingBookings, durationMinutes]);

  return { timeSlots };
}
