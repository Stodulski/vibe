import { Banknote } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function FullPaymentSummary({ estimatedPrice }: { estimatedPrice: number }) {
  return (
    <div className="border-success-border/30 bg-success-bg/30 flex items-center gap-2.5 rounded-lg border px-3 py-2">
      <Banknote className="text-success-text size-4" />
      <span className="text-success-text text-xs">
        {t.bookings.paymentOptions.full}:{' '}
        <span className="score-text font-semibold">{formatPrice(estimatedPrice)}</span>
      </span>
    </div>
  );
}
