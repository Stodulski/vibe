import { PriceBreakdown } from '../booking-form/PriceBreakdown';
import type { BookingPricing } from '../booking-form/pricing';
import { cancellationWindowText } from '../booking-confirmed/cancellationCopy';
import { TicketCourt, TicketPriceRow, TicketStub } from '../ticket/TicketParts';


interface SignUpSummaryCardProps {
  ariaLabel: string;
  /** The complex, on the ticket's first line. */
  venue?: string | undefined;
  /** The day, already written ("Sábado 5 de diciembre"). */
  dateLabel: string;
  /** The hours in large type, when the entry has them. */
  timeLabel?: string | undefined;
  /** What the person signs up for: the event or the tournament. It is the page's `h1`. */
  title: string;
  /** Chips under the title (sport, category, format). */
  tags: string[];
  /** Free lines under the chips (courts, how it is played). */
  details?: string[];
  /** What the entry is called on the first money row: "Lugar", "Inscripción de la pareja". */
  priceLabel: string;
  /** Centavos ARS. */
  price: number;
  serviceFee: number;
  total: number;
  /** The note under the service fee. */
  feeNote: string;
  /** The window the person is told before paying, when the club sets one. */
  cancellationHours?: number | undefined;
}

/**
 * The ticket a booking shows before paying, for an event or a tournament
 * entry: the same green card, tear line, money rows, total band and
 * cancellation sentence, with what is being signed up for in place of the
 * court and the slot.
 */
export function SignUpSummaryCard({
  ariaLabel,
  venue,
  dateLabel,
  timeLabel,
  title,
  tags,
  details = [],
  priceLabel,
  price,
  serviceFee,
  total,
  feeNote,
  cancellationHours,
}: SignUpSummaryCardProps) {
  const pricing: BookingPricing = {
    depositAmount: 0,
    hasDeposit: false,
    mpAmount: price,
    serviceFee,
    totalOnline: total,
    remainingAmount: 0,
  };
  const cancellationWindow = cancellationHours === undefined ? null : cancellationWindowText(cancellationHours);

  return (
    <section aria-label={ariaLabel} className="bg-primary-500 text-text-inverse rounded-3xl p-6 sm:p-8">
      <div className="space-y-6">
        {venue && <p className="text-text-inverse text-sm font-semibold">{venue}</p>}
        <div>
          <p className="text-text-inverse text-sm first-letter:uppercase">{dateLabel}</p>
          {timeLabel && <p className="text-text-inverse text-3xl font-bold tabular-nums">{timeLabel}</p>}
        </div>
        <div className="space-y-2">
          <TicketCourt name={title} tags={tags} as="h1" />
          {details.map((line) => (
            <p key={line} className="text-text-inverse text-sm">
              {line}
            </p>
          ))}
        </div>
      </div>
      <TicketStub />
      <TicketPriceRow label={priceLabel} amount={price} />
      <PriceBreakdown depositPercentage={0} pricing={pricing} feeNote={feeNote} />
      {cancellationWindow && <p className="text-text-inverse mt-6 text-sm">{cancellationWindow}</p>}
    </section>
  );
}
