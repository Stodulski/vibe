import { BOOKING_MESSAGES } from '../../messages';
import { formatPrice, formatDateFull, formatHourRange } from '@vibe/ui';
import { COURT_TYPE_LABELS } from '../court-selector/constants';
import type { BookingInfo } from './types';
import { TicketCourt, TicketStub } from '../ticket/TicketParts';

const t = BOOKING_MESSAGES;
const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;

interface BookingSummaryProps {
  bookingInfo: BookingInfo;
}

// The court's chips: sport and court type. Lines the data
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
    <div className="space-y-6">
      <p className="text-text-inverse text-sm font-semibold">{bookingInfo.complexName}</p>
      <div>
        <p className="text-text-inverse text-sm first-letter:uppercase">{formatDateFull(bookingInfo.date)}</p>
        <p className="text-text-inverse text-3xl font-bold tabular-nums">
          {formatHourRange(bookingInfo.startsAt, bookingInfo.endsAt, `\u00A0${t.publicBooking.timeRangeTo}\u00A0`)}
        </p>
      </div>
      <TicketCourt name={bookingInfo.courtName} tags={courtLines(bookingInfo)} />
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
    <dl className="space-y-3 text-sm">
      <div className="flex justify-between gap-2">
        <dt className="text-text-inverse">{t.publicBooking.paidLabel}</dt>
        <dd className="text-text-inverse tabular-nums">{formatPrice(paidAmount)}</dd>
      </div>
      <div className="flex justify-between gap-2">
        <dt className="text-text-inverse">{t.publicBooking.oweAtClubLabel}</dt>
        <dd className="text-text-inverse font-bold tabular-nums">{formatPrice(remaining)}</dd>
      </div>
    </dl>
  );
}

export function BookingSummary({ bookingInfo }: BookingSummaryProps) {
  return (
    <section
      aria-label="Detalle de la reserva"
      className="bg-primary-500 text-text-inverse w-full rounded-3xl p-6 sm:p-8 text-left"
    >
      <TicketHeader bookingInfo={bookingInfo} />
      <TicketStub />
      <MoneySummary bookingInfo={bookingInfo} />
    </section>
  );
}
