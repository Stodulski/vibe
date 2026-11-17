import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryStartLabel } from '@/shared/lib/traitLabels';
import { formatPrice } from '@/shared/lib/utils';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** Joins the facts that are there: a fact with no value (no start date yet) is left out. */
function factsLine(facts: (string | null)[]): string {
  return facts.filter((fact): fact is string => !!fact).join(' · ');
}

/**
 * The category in one line of key facts. From `sm` up it reads in full: pairs taken,
 * price per pair, format and start. A phone reads it short, "8/8 · $30.000 · Zonas",
 * without the start, so the line fits on one row at 256px.
 */
export function CategoryFacts({ category }: { category: TournamentOwnerCategory }) {
  const pairs = t.detail.pairsRatio(category.confirmed_count, category.capacity);
  const pairsShort = t.detail.pairsRatioShort(category.confirmed_count, category.capacity);
  const price = formatPrice(category.price);
  const format = t.format[category.format];
  const start = categoryStartLabel(category.starts_on, category.starts_at);
  const summaryFull = factsLine([pairs, t.pricePerPair(price), format, start]);
  const summaryShort = factsLine([pairsShort, price, t.formatShort[category.format]]);

  return (
    <div className="min-w-0 flex-1 space-y-2">
      <p className="text-text-secondary truncate text-sm" title={summaryFull}>
        <span className="sm:hidden">{summaryShort}</span>
        <span className="hidden sm:inline">{summaryFull}</span>
      </p>
      {category.prizes && (
        <p className="text-text-tertiary truncate text-xs" title={category.prizes}>
          {category.prizes}
        </p>
      )}
    </div>
  );
}
