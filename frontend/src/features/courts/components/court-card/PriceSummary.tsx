import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPriceLabel, type PriceRange } from './priceRange';

const t = ES_AR;

/**
 * The two rates, as data rather than as containers.
 *
 * They used to be two filled boxes, which made the loudest thing on the card
 * the one carrying the least exact fact — a derived range. On a grid of a
 * dozen courts the eye landed on twelve near-identical grey rectangles instead
 * of on the names it was looking for. Now they read like the line above them:
 * label, value, nothing drawn around either.
 */
export function PriceSummary({ weekdayMin, weekdayMax, weekendMin, weekendMax, hasPrices }: PriceRange) {
  if (!hasPrices) {
    // Same footprint as the two rate lines below, so a court without prices
    // does not make its card shorter than the one beside it. `min-h` is the
    // two-line stack: 2 × 20px line box + the 4px gap between them.
    return <p className="text-text-tertiary min-h-11 text-sm">{t.courts.noPrices}</p>;
  }

  return (
    <dl className="space-y-1">
      <PriceLine label={t.courts.weekdaysShort} value={formatPriceLabel(weekdayMin, weekdayMax, formatPrice)} />
      <PriceLine label={t.courts.weekendShort} value={formatPriceLabel(weekendMin, weekendMax, formatPrice)} />
    </dl>
  );
}

/** Fixed label column, so both values line up down the card. */
function PriceLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="text-text-tertiary w-16 shrink-0 text-xs whitespace-nowrap">{label}</dt>
      {/* The rate in the brand green, its day label left neutral. The number is
          what the owner came to read; the label only says which one it is. */}
      <dd className="score-text text-primary-400 min-w-0 truncate text-sm font-semibold whitespace-nowrap">{value}</dd>
    </div>
  );
}
