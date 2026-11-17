import type { BookingSlotInfo } from './types';
import type { BookingPricing } from './pricing';
import { SlotHeaderRow } from './SlotHeaderRow';
import { PriceBreakdown } from './PriceBreakdown';
import { cancellationWindowText } from '../booking-confirmed/cancellationCopy';

interface BookingSummaryCardProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
  /** Inside the checkout's green band, which already is the card: no surface, rounding or padding of its own. */
  bare?: boolean;
}

const CARD_SURFACE = 'bg-primary-500 text-text-inverse rounded-3xl p-6 sm:p-8';

export function BookingSummaryCard({ slotInfo, pricing, bare = false }: BookingSummaryCardProps) {
  // The policy is stated here, before the pay button, so the customer reads
  // the window they are agreeing to before they commit the money.
  const cancellationWindow = cancellationWindowText(slotInfo.cancellationHours);

  return (
    // A solid green card, the same ticket as the confirmed booking, unless it
    // is bare inside a band. The ink inside it is near-black, so the text stays
    // legible on the brand green.
    <section aria-label="Resumen de la reserva" className={bare ? 'text-text-inverse' : CARD_SURFACE}>
      <SlotHeaderRow slotInfo={slotInfo} bare={bare} />
      <PriceBreakdown depositPercentage={slotInfo.depositPercentage} pricing={pricing} />
      {cancellationWindow && <p className="text-text-inverse mt-6 text-sm">{cancellationWindow}</p>}
    </section>
  );
}
