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
    // No card on phones: the bands and their hairlines already group the
    // summary, and the border only ate width. The card returns from sm up.
    <section
      aria-label="Resumen de la reserva"
      className="sm:border-border-subtle sm:bg-bg-subtle sm:rounded-2xl sm:border sm:p-5"
    >
      <SlotHeaderRow slotInfo={slotInfo} />
      <PriceBreakdown depositPercentage={slotInfo.depositPercentage} pricing={pricing} />
      {cancellationWindow && <p className="text-text-secondary mt-3 text-sm">{cancellationWindow}</p>}
    </section>
  );
}
