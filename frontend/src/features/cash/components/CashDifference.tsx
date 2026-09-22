import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { cn } from '@/shared/lib/utils';
import { cashDifferenceLabel } from '../lib/cashDifferenceLabel';

const t = ES_AR;

/**
 * `counted - expected`, labelled sobrante/faltante/sin diferencia — shared by
 * the close dialog's LIVE preview (as the amount is typed) and the closed
 * result's final, server-committed number.
 */
export function CashDifference({ difference }: { difference: number }) {
  const { label, colorClass } = cashDifferenceLabel(difference);

  return (
    <div className="bg-bg-base flex items-center justify-between rounded-xl px-3.5 py-2.5 text-sm">
      <span className="text-text-tertiary">{t.cash.difference}</span>
      <span className={cn('font-semibold', colorClass)}>
        {label}
        {difference !== 0 && `: ${formatPrice(Math.abs(difference))}`}
      </span>
    </div>
  );
}
