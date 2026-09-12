import { DollarSign } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function PricePreview({ estimatedPrice }: { estimatedPrice: number }) {
  return (
    <div className="border-primary-500/10 bg-primary-500/[0.04] flex items-center gap-3 rounded-xl border px-4 py-3">
      <div className="bg-primary-500/10 flex size-8 items-center justify-center rounded-lg">
        <DollarSign className="text-primary-400 size-4" />
      </div>
      <div>
        <p className="text-text-tertiary text-xs font-medium">{t.bookings.price}</p>
        <p className="score-text text-text-primary text-lg font-bold">{formatPrice(estimatedPrice)}</p>
      </div>
    </div>
  );
}
