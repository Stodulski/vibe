import { timeToMinutes } from '@/shared/lib/time';
import { overlaps as spansOverlap, spanOnDay, toSpan, type Span } from '@/shared/lib/instants';
import type { Booking, BlockedSlot } from '@/shared/types/api.types';

/**
 * Whether a court is free at a given moment, asked the same way everywhere.
 *
 * This lives in `shared` rather than inside the booking calendar because two
 * features ask it: the owner's grid, deciding which slots to offer for a new
 * booking, and the block-slot form, deciding which slots may be blocked. They
 * were answering it differently, and one of them was wrong — the block form
 * compared minutes-of-day, so a booking running 23:00 to 01:00 asked
 * `slotMin >= 1380 && slotMin < 60`, which no minute of any day satisfies. The
 * booking became invisible and its hours were offered as free to block.
 *
 * A question two features ask about the same data has to have one answer.
 */

/**
 * The spans that put a court out of use on `date`, from both kinds of obstacle.
 *
 * Bookings carry their own instants, so one that began yesterday keeps holding
 * the court this morning. Blocked slots are still a date plus two times of day,
 * which is exact for them: `blocked_slots` has required start < end since the
 * schema's first migration, so a block never leaves its date.
 */
export function courtObstacles(
  bookings: Booking[],
  blockedSlots: BlockedSlot[],
  courtId: string,
  date: string,
): Span[] {
  const spans: Span[] = [];
  for (const b of bookings) {
    if (b.court_id !== courtId || b.status === 'cancelled') continue;
    const span = toSpan(b.starts_at, b.ends_at);
    if (span) spans.push(span);
  }
  for (const s of blockedSlots) {
    if (s.court_id !== courtId) continue;
    const span = spanOnDay(date, timeToMinutes(s.start_time), timeToMinutes(s.end_time));
    if (span) spans.push(span);
  }
  return spans;
}

/**
 * True when anything overlaps `[startMin, startMin + lengthMin)` on `date`.
 *
 * A real interval overlap, not a sample of the window's first instant, which
 * would miss anything starting inside it. This mirrors the overlap test the
 * server applies when it accepts a booking, so the two cannot disagree about
 * whether a slot is free.
 */
export function windowIsOccupied(obstacles: Span[], date: string, startMin: number, lengthMin: number): boolean {
  const window = spanOnDay(date, startMin, startMin + lengthMin);
  if (!window) return false;

  return obstacles.some((o) => spansOverlap(window.start, window.end, o.start, o.end));
}
