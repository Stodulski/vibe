import type { BookingSlotInfo } from './types';
import type { BookingPricing } from './pricing';
import { SlotHeaderRow } from './SlotHeaderRow';
import { PriceBreakdown } from './PriceBreakdown';
import { cancellationWindowText } from '../booking-confirmed/cancellationCopy';

interface BookingSummaryCardProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
}

export function BookingSummaryCard({ slotInfo, pricing }: BookingSummaryCardProps) {
  // The policy is stated here, before the pay button, so the customer reads
  // the window they are agreeing to before they commit the money.
  const cancellationWindow = cancellationWindowText(slotInfo.cancellationHours);

  return (
    // A solid green card at every width, the same ticket as the confirmed
    // booking. The ink inside it is near-black, so the text stays legible on
    // the brand green.
    <section aria-label="Resumen de la reserva" className="bg-primary-500 text-text-inverse rounded-3xl p-5">
      <SlotHeaderRow slotInfo={slotInfo} />
      <PriceBreakdown depositPercentage={slotInfo.depositPercentage} pricing={pricing} />
      {cancellationWindow && <p className="text-text-inverse/80 mt-3 text-sm">{cancellationWindow}</p>}
    </section>
  );
}
