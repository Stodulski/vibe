import { minutesInto, toSpan } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import { cn, formatHourRange } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { STATUS_BAR_STYLES } from '../BookingStatusBadge';
import { barPosition } from './timelineGeometry';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import { paymentDisplayStatus } from '@/shared/types/api.types';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

// A block needs both lines (icon+time, then name) once it's at least this
// tall; below that only the name fits. Derived from the booking's own
// duration, never from a measured DOM box — jsdom has no layout to measure.
const TWO_LINE_MIN_HEIGHT_PX = 56;

/**
 * The booking's position on `date`, in minutes from that day's midnight.
 *
 * Falls back to the day-and-duration fields when the instants are missing or
 * unreadable — a response cached before the server began sending them. The
 * fallback used to read the stored end as well, and that end was a clock
 * reading: for a 23:00 booking of two hours it said "01:00", so the block was
 * drawn as running backwards and a guard for that treated it as corrupt data.
 * Adding the duration to the start cannot wrap, so the fallback now draws the
 * overnight case correctly too.
 */
function spanMinutes(booking: Booking, date: string): { startMin: number; endMin: number } {
  const span = toSpan(booking.starts_at, booking.ends_at);
  if (span) {
    const startMin = minutesInto(date, span.start);
    const endMin = minutesInto(date, span.end);
    if (startMin !== null && endMin !== null) return { startMin, endMin };
  }
  const startMin = timeToMinutes(booking.start_time);
  return { startMin, endMin: startMin + booking.duration_minutes };
}

// Court and client names are read out bare: prefixing them with their field
// label produced "Cancha Cancha 1, Cliente Juan Perez" — the courts are
// already named "Cancha N", and a person's name needs no announcement.
function bookingAriaLabel(booking: Booking, courtName: string): string {
  return [
    courtName,
    formatHourRange(booking.starts_at, booking.ends_at, '–'),
    booking.client_name,
    `${t.bookings.status}: ${t.bookings.statuses[booking.status]}`,
    `${t.bookings.paymentStatus}: ${t.bookings.paymentStatuses[paymentDisplayStatus(booking)]}`,
  ].join(', ');
}

interface BookingBlockProps {
  booking: Booking;
  courtName: string;
  /** The calendar day this column is showing, as `YYYY-MM-DD`. */
  date: string;
  onSelect: () => void;
}

/** A booking's stretch of its court's column, filled in its status colour. */
export function BookingBlock({ booking, courtName, date, onSelect }: BookingBlockProps) {
  // Measured against the day being displayed, from the booking's own instants.
  // A booking that began yesterday gives a negative start and one ending
  // tomorrow gives more than a full day; `barPosition` clips both to the grid's
  // edge, which is what draws the visible half of an overnight booking in the
  // right place. Reading the pair of clock readings instead put a 23:00-01:00
  // booking at 23:00 with a 30-minute height, because "01:00" is the smaller
  // number and the guard for that treated it as corrupt data.
  const { startMin, endMin } = spanMinutes(booking, date);
  // `barPosition`'s `leftPct`/`widthPct` are orientation-agnostic percentages
  // of the range span; aliased here to `top`/`height` for the vertical grid.
  const {
    leftPct: topPct,
    widthPct: heightPct,
    visibleMin,
  } = barPosition(startMin, endMin, DAY_START_MIN, DAY_END_MIN);
  // From the clipped minutes, not from the clipped percentage. Both describe
  // the same stretch — that is the point, so height and position cannot
  // disagree about how much of an overnight booking this day shows — but only
  // the minutes are exact. Going through the percentage turned a 60-minute
  // block into 55.999999999999986px and lost its time label.
  const heightPx = (visibleMin / SLOT_MINUTES) * SLOT_HEIGHT_PX;
  const showBoth = heightPx >= TWO_LINE_MIN_HEIGHT_PX;
  // A booking that leaves this day is drawn square on the edge it crosses.
  // Rounded on all four corners it reads as a booking that simply ended there,
  // which is the one thing it did not do — the rest of it is on the next day's
  // grid, where the same rule squares the top.
  const runsOnPastToday = endMin > DAY_END_MIN;
  const beganBeforeToday = startMin < DAY_START_MIN;
  const styles = STATUS_BAR_STYLES[booking.status];

  const ariaLabel = bookingAriaLabel(booking, courtName);

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect();
        }
      }}
      className={cn(
        'focus-self absolute inset-x-[3px] flex cursor-pointer flex-col overflow-hidden rounded-md border px-1.5 py-0.5',
        'transition-[filter] focus-visible:brightness-150',
        runsOnPastToday && 'rounded-b-none border-b-0',
        beganBeforeToday && 'rounded-t-none border-t-0',
        styles.bg,
        styles.border,
        booking.status === 'cancelled' && 'opacity-40',
      )}
      // Inset 1px per side on the time axis: two back-to-back bookings share
      // an edge, and two 1px borders meeting there read as a doubled line
      // instead of a seam.
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {showBoth && (
        // No status icon here: the block's own fill already carries the state,
        // and the accessible name spells it out for anyone who can't see it.
        //
        // Dimmed white rather than a grey token: the greys are calibrated
        // against the near-black page, and on a mid-tone fill they drop to
        // ~2:1. This holds 5.1:1 and still reads as the secondary line.
        <span className="score-text text-[0.6875rem] text-text-primary/80">
          {formatHourRange(booking.starts_at, booking.ends_at)}
        </span>
      )}
      <p className="truncate text-xs font-medium text-text-primary">{booking.client_name}</p>
    </div>
  );
}
