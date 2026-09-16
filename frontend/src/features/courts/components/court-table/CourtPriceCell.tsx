import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPriceLabel, getOverallPriceRange } from '../court-card/priceRange';
import type { CourtPrice } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * One general range across every price band, regardless of day — what the
 * table's single "Precio" column shows instead of `PriceSummary`'s separate
 * weekday/weekend lines. Empty state matches the card's own wording.
 */
export function CourtPriceCell({ prices }: { prices: CourtPrice[] }) {
  const range = getOverallPriceRange(prices);

  if (!range.hasPrices) {
    return <span className="text-text-tertiary text-sm whitespace-nowrap">{t.courts.noPrices}</span>;
  }

  return (
    <span className="score-text text-primary-400 text-sm font-semibold whitespace-nowrap">
      {formatPriceLabel(range.min, range.max, formatPrice)}
    </span>
  );
}
