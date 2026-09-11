import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, formatDateFull, formatHourRange } from '@/shared/lib/utils';
import { COURT_TYPE_LABELS } from '../court-selector/constants';
import type { BookingInfo } from './types';

const t = ES_AR;
const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;

interface BookingSummaryProps {
  bookingInfo: BookingInfo;
}

// The court block, one line each: the court name as the heading, then the
// sport and the court type as plain descriptors underneath. Lines the data
// lacks (an older API or a cache without sport/type) are simply not rendered.
function courtLines(bookingInfo: BookingInfo): string[] {
  const sport = bookingInfo.sport && (SPORT_LABELS[bookingInfo.sport] ?? bookingInfo.sport);
  const courtType = bookingInfo.courtType && (COURT_TYPE_LABELS[bookingInfo.courtType] ?? bookingInfo.courtType);
  return [sport, courtType].filter((line): line is string => Boolean(line));
}

// The ticket's top half: date, a big time range, and what was booked. Name
// only — the address was on the complex page and travels in the WhatsApp
// message the complex sends, so repeating it here is noise.
function TicketHeader({ bookingInfo }: BookingSummaryProps) {
  return (
    <div className="divide-y divide-border-subtle">
      <p className="pb-3 text-sm font-semibold text-text-primary">{bookingInfo.complexName}</p>
      <div className="py-3">
        <p className="text-sm text-text-secondary first-letter:uppercase">{formatDateFull(bookingInfo.date)}</p>
        <p className="text-3xl font-bold tabular-nums text-text-primary">
          {formatHourRange(bookingInfo.startsAt, bookingInfo.endsAt, `\u00A0${t.publicBooking.timeRangeTo}\u00A0`)}
        </p>
      </div>
      <div className="space-y-1 pt-3">
        <p className="text-base font-semibold text-text-primary">{bookingInfo.courtName}</p>
        {courtLines(bookingInfo).map((line) => (
          <p key={line} className="text-sm text-text-secondary">
            {line}
          </p>
        ))}
      </div>
    </div>
  );
}

// The ticket-stub divider between the details and the money: a dashed rule
// with two notches carved out of the card's own left/right edges, filled
// with the page's background colour, so it reads as a tear line rather than
// a plain rule cutting the card in half.
function TicketStub() {
  return (
    <div className="relative my-4 sm:-mx-5">
      <div className="border-t border-dashed border-border-subtle" />
      <span
        aria-hidden="true"
        className="absolute left-0 top-1/2 hidden size-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-bg-base sm:block"
      />
      <span
        aria-hidden="true"
        className="absolute right-0 top-1/2 hidden size-3 -translate-y-1/2 translate-x-1/2 rounded-full bg-bg-base sm:block"
      />
    </div>
  );
}

// The money as a two-row summary, not a sentence: what already moved (deposit
// plus the service fee, summed: the split was shown before paying and a
// refund inside the window returns both) and, in bold, what is still owed at
// the venue — 0 once nothing is left.
function MoneySummary({ bookingInfo }: BookingSummaryProps) {
  const paidAmount = bookingInfo.depositAmount + (bookingInfo.serviceFee ?? 0);
  const remaining = bookingInfo.remainingAmount ?? bookingInfo.price - bookingInfo.depositAmount;

  return (
    <dl className="divide-y divide-border-subtle border-b border-border-subtle text-sm sm:border-b-0">
      <div className="flex justify-between gap-2 pb-3">
        <dt className="text-text-secondary">{t.publicBooking.paidLabel}</dt>
        <dd className="tabular-nums text-text-primary">{formatPrice(paidAmount)}</dd>
      </div>
      <div className="flex justify-between gap-2 py-3">
        <dt className="text-text-secondary">{t.publicBooking.oweAtClubLabel}</dt>
        <dd className="font-bold tabular-nums text-text-primary">{formatPrice(remaining)}</dd>
      </div>
    </dl>
  );
}

export function BookingSummary({ bookingInfo }: BookingSummaryProps) {
  return (
    <section
      aria-label="Detalle de la reserva"
      className="w-full text-left sm:rounded-2xl sm:border sm:border-border-subtle sm:bg-bg-subtle sm:p-5"
    >
      <TicketHeader bookingInfo={bookingInfo} />
      <TicketStub />
      <MoneySummary bookingInfo={bookingInfo} />
    </section>
  );
}
