import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateFull, formatHourRange } from '@/shared/lib/utils';
import { COURT_TYPE_LABELS } from '@/features/public-booking';
import type { CancelInfoResponse } from '@/shared/types/api.types';

const t = ES_AR;
const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;

type CancelInfoBooking = CancelInfoResponse['booking'];

// The money band under the booking: either a labelled amount (what comes
// back, or what was paid and will not), or just a sentence when there is no
// figure to show (a refund the venue makes by hand).
export interface CancelMoneyBand {
  label?: string;
  amount?: string;
  note?: string;
}

interface BookingInfoCardProps {
  cancelInfo: CancelInfoResponse;
  money?: CancelMoneyBand | null;
}

// Sport and court type as their own lines under the court name, the same way
// the confirmed booking's ticket and the court cards show them. Lines the
// data lacks (an older API) are simply not rendered.
function courtLines(booking: CancelInfoBooking): string[] {
  const sport = booking.sport && (SPORT_LABELS[booking.sport] ?? booking.sport);
  const courtType = booking.court_type && (COURT_TYPE_LABELS[booking.court_type] ?? booking.court_type);
  return [sport, courtType].filter((line): line is string => Boolean(line));
}

// The two instants arrived with the cancel-info extension; an older server
// answers with just `start_time`, so the range falls back to the start alone.
//
// They replaced a pair of clock readings, and this is one of the six screens
// where that pair lied: a booking running 23:00 to 01:00 was shown ending two
// hours before it began, on the page where someone decides whether to cancel it.
function timeRange(booking: CancelInfoBooking): string {
  if (!booking.starts_at || !booking.ends_at) return booking.start_time;
  return formatHourRange(booking.starts_at, booking.ends_at, `\u00A0${t.publicBooking.timeRangeTo}\u00A0`);
}

// The booking being cancelled, in the same bands as the confirmed booking's
// ticket so the person recognises what they saw confirmed: venue, day and
// hours in large type, court with sport and type. Hairlines between bands and a
// closing one under the last on phones; from sm the card border closes it.
export function BookingInfoCard({ cancelInfo, money }: BookingInfoCardProps) {
  const { booking } = cancelInfo;
  return (
    <section
      aria-label="Detalle de la reserva"
      className="divide-border-subtle border-border-subtle sm:border-border-subtle sm:bg-bg-subtle w-full divide-y border-b text-left sm:rounded-2xl sm:border sm:p-5"
    >
      <p className="text-text-primary pb-3 text-sm font-semibold">{booking.complex_name}</p>
      <div className="py-3">
        <p className="text-text-secondary text-sm first-letter:uppercase">
          {formatDateFull(`${booking.date}T12:00:00`)}
        </p>
        <p className="text-text-primary text-3xl font-bold tabular-nums">{timeRange(booking)}</p>
      </div>
      <div className="space-y-1 py-3">
        <p className="text-text-primary text-base font-semibold">{booking.court_name}</p>
        {courtLines(booking).map((line) => (
          <p key={line} className="text-text-secondary text-sm">
            {line}
          </p>
        ))}
      </div>
      {money && (
        <div className="py-3 text-sm">
          {money.label && (
            <div className="flex justify-between gap-2">
              <span className="text-text-secondary">{money.label}</span>
              <span className="text-text-primary font-bold tabular-nums">{money.amount}</span>
            </div>
          )}
          {money.note && (
            <p className={money.label ? 'text-text-tertiary mt-1 text-xs' : 'text-text-secondary'}>{money.note}</p>
          )}
        </div>
      )}
    </section>
  );
}
