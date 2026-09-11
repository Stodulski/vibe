import { DollarSign } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function PricePreview({ estimatedPrice }: { estimatedPrice: number }) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-primary-500/10 bg-primary-500/[0.04] px-4 py-3">
      <div className="flex size-8 items-center justify-center rounded-lg bg-primary-500/10">
        <DollarSign className="size-4 text-primary-400" />
      </div>
      <div>
        <p className="text-xs font-medium text-text-tertiary">{t.bookings.price}</p>
        <p className="score-text text-lg font-bold text-text-primary">{formatPrice(estimatedPrice)}</p>
      </div>
    </div>
  );
}
