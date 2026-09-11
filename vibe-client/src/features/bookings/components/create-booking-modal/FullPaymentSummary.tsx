import { Banknote } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function FullPaymentSummary({ estimatedPrice }: { estimatedPrice: number }) {
  return (
    <div className="flex items-center gap-2.5 rounded-lg border border-success-border/30 bg-success-bg/30 px-3 py-2">
      <Banknote className="size-4 text-success-text" />
      <span className="text-xs text-success-text">
        {t.bookings.paymentOptions.full}:{' '}
        <span className="score-text font-semibold">{formatPrice(estimatedPrice)}</span>
      </span>
    </div>
  );
}
