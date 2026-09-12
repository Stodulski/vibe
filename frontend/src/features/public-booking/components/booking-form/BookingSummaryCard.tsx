import type { BookingSlotInfo } from './types';
import type { BookingPricing } from './pricing';
import { SlotHeaderRow } from './SlotHeaderRow';
import { PriceBreakdown } from './PriceBreakdown';

interface BookingSummaryCardProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
}

export function BookingSummaryCard({ slotInfo, pricing }: BookingSummaryCardProps) {
  return (
    // No card on phones: the bands and their hairlines already group the
    // summary, and the border only ate width. The card returns from sm up.
    <section
      aria-label="Resumen de la reserva"
      className="sm:border-border-subtle sm:bg-bg-subtle sm:rounded-2xl sm:border sm:p-5"
    >
      <SlotHeaderRow slotInfo={slotInfo} />
      <PriceBreakdown depositPercentage={slotInfo.depositPercentage} pricing={pricing} />
    </section>
  );
}
