import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { BookingPricing } from './pricing';

const t = ES_AR;

interface PriceBreakdownProps {
  depositPercentage: number;
  pricing: BookingPricing;
}

// The money rows, one band each with a hairline between them, continuing the
// bands SlotHeaderRow opened above: the deposit, the service fee with the
// note that it is not deducted from the court price, what is paid now in
// bold, and what is still owed at the venue.
export function PriceBreakdown({ depositPercentage, pricing }: PriceBreakdownProps) {
  const { hasDeposit, mpAmount, serviceFee, totalOnline, remainingAmount } = pricing;

  return (
    <dl className="divide-y divide-border-subtle border-t border-b border-border-subtle text-sm sm:border-b-0">
      {hasDeposit && (
        <div className="flex justify-between gap-2 py-3">
          <dt className="text-text-secondary">
            {t.publicBooking.deposit} ({depositPercentage}%)
          </dt>
          <dd className="tabular-nums text-text-primary">{formatPrice(mpAmount)}</dd>
        </div>
      )}
      <div className="py-3">
        <div className="flex justify-between gap-2">
          <dt className="text-text-secondary">{t.serviceFee.label}</dt>
          <dd className="tabular-nums text-text-primary">{formatPrice(serviceFee)}</dd>
        </div>
        <p className="mt-1 text-xs text-text-tertiary">{t.serviceFee.notDeductedFromCourtPrice}</p>
      </div>
      <div className="flex items-baseline justify-between gap-2 py-3">
        <dt className="font-semibold text-text-primary">{t.publicBooking.totalOnline}</dt>
        <dd className="text-xl font-bold tabular-nums text-primary-400">{formatPrice(totalOnline)}</dd>
      </div>
      {remainingAmount > 0 && (
        <div className="flex justify-between gap-2 py-3">
          <dt className="text-text-secondary">{t.publicBooking.remaining}</dt>
          <dd className="font-bold tabular-nums text-text-primary">{formatPrice(remainingAmount)}</dd>
        </div>
      )}
    </dl>
  );
}
