import { HOURS_ANCHOR } from '../../components/CourtSelector';
import type { BookingInfo } from '../../components/booking-confirmed/types';
import { publicComplexPath } from '../../lib/publicPaths';

/**
 * Where "Elegir otro horario" lands after a rejected payment: the hours of the
 * complex page on the day of the slot that failed, with its duration and sport
 * when known, the way "Cambiar horario" lands on the confirm page
 * (`hoursPath`). Without the stored slot it is the complex page itself.
 */
export function failedSlotPath(slug: string, booking: BookingInfo | null): string {
  if (!booking) return publicComplexPath(slug);
  const query = new URLSearchParams({ date: booking.date });
  const minutes = (Date.parse(booking.endsAt) - Date.parse(booking.startsAt)) / 60_000;
  if (Number.isInteger(minutes) && minutes > 0) query.set('duration', String(minutes));
  if (booking.sport) query.set('sport', booking.sport);
  return `${publicComplexPath(slug)}?${query.toString()}${HOURS_ANCHOR}`;
}
